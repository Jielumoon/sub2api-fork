package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/upstream"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

// upstreamRepository 上游实体仓储。
//
// 探测结果的写回走原生 SQL：它必须在同一条语句里比较 config_version 与代理身份，
// 这是 Ent builder 不便表达的 CAS。普通 CRUD 用 Ent 以复用软删除拦截器。
type upstreamRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewUpstreamRepository(client *dbent.Client, sqlDB *sql.DB) service.UpstreamRepository {
	return newUpstreamRepositoryWithSQL(client, sqlDB)
}

func newUpstreamRepositoryWithSQL(client *dbent.Client, sqlq sqlExecutor) *upstreamRepository {
	return &upstreamRepository{client: client, sql: sqlq}
}

// updater 复用已有事务上下文：普通查询用 Ent（软删除由拦截器负责），
// 需要 CAS 的原生语句用同一事务里的 SQL 执行器，避免写到事务之外。
func (r *upstreamRepository) updater(ctx context.Context) sqlExecutor {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.sql
}

func (r *upstreamRepository) Create(ctx context.Context, in *service.Upstream) error {
	created, err := r.client.Upstream.Create().
		SetName(in.Name).
		SetBaseURL(in.BaseURL).
		SetAPIKeyEncrypted(in.APIKeyEncrypted).
		SetNillableProxyID(in.ProxyID).
		SetEnabled(in.Enabled).
		SetIntervalMinutes(in.IntervalMinutes).
		SetConfigVersion(in.ConfigVersion).
		SetFailureCount(0).
		SetLastError("").
		Save(ctx)
	if err != nil {
		return err
	}
	applyUpstreamEntity(in, created)
	return nil
}

func (r *upstreamRepository) GetByID(ctx context.Context, id int64) (*service.Upstream, error) {
	m, err := r.client.Upstream.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrUpstreamNotFound
		}
		return nil, err
	}
	return upstreamEntityToService(m), nil
}

func (r *upstreamRepository) List(ctx context.Context, params pagination.PaginationParams, search string) ([]service.Upstream, *pagination.PaginationResult, error) {
	q := r.client.Upstream.Query()
	if search != "" {
		q = q.Where(upstream.Or(upstream.NameContainsFold(search), upstream.BaseURLContainsFold(search)))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(upstream.ByID(entsql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.Upstream, 0, len(rows))
	for _, row := range rows {
		out = append(out, *upstreamEntityToService(row))
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

// ListDue 到期扫描：next_probe_at 为空视为立即到期，NULL 排在最前。
func (r *upstreamRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]service.Upstream, error) {
	if limit <= 0 {
		return []service.Upstream{}, nil
	}
	rows, err := r.client.Upstream.Query().
		Where(
			upstream.EnabledEQ(true),
			upstream.Or(upstream.NextProbeAtIsNil(), upstream.NextProbeAtLTE(now)),
		).
		Order(upstream.ByNextProbeAt(entsql.OrderAsc(), entsql.OrderNullsFirst())).
		Order(upstream.ByID(entsql.OrderAsc())).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.Upstream, 0, len(rows))
	for _, row := range rows {
		out = append(out, *upstreamEntityToService(row))
	}
	return out, nil
}

// Update 写入管理端提交的配置；地址／Key／代理与库内旧值不同即换配置版本并清空快照。
// 比较在同一 UPDATE 内完成（SET 右侧读到的都是旧行值），避免并发编辑写入陈旧副本。
func (r *upstreamRepository) Update(ctx context.Context, in service.UpstreamUpdate) (bool, error) {
	var apiKey any
	if in.APIKeyEncrypted != nil {
		apiKey = *in.APIKeyEncrypted
	}
	var proxyID any
	if in.ProxyID != nil {
		proxyID = *in.ProxyID
	}
	// $8 为本次候选配置版本（新 UUID），只有确认是配置变更时才写入；
	// RETURNING 看到的是新行，所以用「版本已变成候选值」判断本次是否改了配置。
	refresh := `(base_url IS DISTINCT FROM $3::text
			OR ($4::text IS NOT NULL AND api_key_encrypted IS DISTINCT FROM $4::text)
			OR proxy_id IS DISTINCT FROM $5::bigint)`
	var changed bool
	rows, err := r.updater(ctx).QueryContext(ctx, fmt.Sprintf(`
		UPDATE upstreams SET
			name = $2,
			base_url = $3::text,
			api_key_encrypted = COALESCE($4::text, api_key_encrypted),
			proxy_id = $5::bigint,
			enabled = $6,
			interval_minutes = $7,
			config_version = CASE WHEN %[1]s THEN $8::text ELSE config_version END,
			probe = CASE WHEN %[1]s THEN NULL ELSE probe END,
			next_probe_at = CASE WHEN %[1]s THEN NULL ELSE next_probe_at END,
			failure_count = CASE WHEN %[1]s THEN 0 ELSE failure_count END,
			last_error = CASE WHEN %[1]s THEN '' ELSE last_error END,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING config_version = $8::text
	`, refresh), in.ID, in.Name, in.BaseURL, apiKey, proxyID, in.Enabled, in.IntervalMinutes, in.NewConfigVersion)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, service.ErrUpstreamNotFound
	}
	if err := rows.Scan(&changed); err != nil {
		return false, err
	}
	return changed, rows.Err()
}

// Delete 软删除并清理快照与调度列，避免删除后仍被到期扫描选中。
func (r *upstreamRepository) Delete(ctx context.Context, id int64) error {
	result, err := clientFromContext(ctx, r.client).ExecContext(ctx, `
		UPDATE upstreams SET
			deleted_at = NOW(),
			probe = NULL,
			next_probe_at = NULL,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrUpstreamNotFound
	}
	return nil
}

// UpdateProbeResult CAS 写回快照：配置版本或代理身份变化即拒绝，返回 ErrUpstreamProbeIdentityChanged。
func (r *upstreamRepository) UpdateProbeResult(
	ctx context.Context,
	id int64,
	configVersion string,
	proxyID *int64,
	proxy *service.Proxy,
	snapshot *service.UpstreamProbeSnapshot,
) error {
	if snapshot == nil {
		return errors.New("upstream probe snapshot is nil")
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	// 代理行的 FOR SHARE 锁只在事务内有效：没有外层事务时自己开一个，
	// 让「核对代理身份」与「写回快照」之间代理不能被修改。
	if dbent.TxFromContext(ctx) == nil {
		tx, txErr := r.client.Tx(ctx)
		if txErr != nil && !errors.Is(txErr, dbent.ErrTxStarted) {
			return txErr
		}
		if txErr == nil {
			defer func() { _ = tx.Rollback() }()
			if err := r.updateProbeResultInTx(dbent.NewTxContext(ctx, tx), id, configVersion, proxyID, proxy, snapshot, payload); err != nil {
				return err
			}
			return tx.Commit()
		}
	}
	return r.updateProbeResultInTx(ctx, id, configVersion, proxyID, proxy, snapshot, payload)
}

func (r *upstreamRepository) updateProbeResultInTx(
	ctx context.Context,
	id int64,
	configVersion string,
	proxyID *int64,
	proxy *service.Proxy,
	snapshot *service.UpstreamProbeSnapshot,
	payload []byte,
) error {
	client := clientFromContext(ctx, r.client)
	matches, err := lockAndMatchProxyIdentity(ctx, client, proxyID, proxy, nil)
	if err != nil {
		return err
	}
	if !matches {
		return service.ErrUpstreamProbeIdentityChanged
	}
	lastError := snapshot.LastError
	if len(lastError) > 100 {
		lastError = lastError[:100]
	}
	result, err := client.ExecContext(ctx, `
		UPDATE upstreams SET
			probe = $1::jsonb,
			next_probe_at = $2,
			last_attempt_at = $3,
			failure_count = $4,
			last_error = $5,
			updated_at = NOW()
		WHERE id = $6 AND deleted_at IS NULL AND config_version = $7
	`, string(payload), snapshot.NextProbeAt, snapshot.LastAttemptAt, snapshot.FailureCount, lastError, id, configVersion)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrUpstreamProbeIdentityChanged
	}
	return nil
}

func upstreamEntityToService(m *dbent.Upstream) *service.Upstream {
	if m == nil {
		return nil
	}
	out := &service.Upstream{
		ID:              m.ID,
		Name:            m.Name,
		BaseURL:         m.BaseURL,
		APIKeyEncrypted: m.APIKeyEncrypted,
		ProxyID:         m.ProxyID,
		Enabled:         m.Enabled,
		IntervalMinutes: m.IntervalMinutes,
		ConfigVersion:   m.ConfigVersion,
		NextProbeAt:     m.NextProbeAt,
		LastAttemptAt:   m.LastAttemptAt,
		FailureCount:    m.FailureCount,
		LastError:       m.LastError,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if len(m.Probe) > 0 {
		raw, err := json.Marshal(m.Probe)
		if err == nil {
			var snapshot service.UpstreamProbeSnapshot
			if json.Unmarshal(raw, &snapshot) == nil && snapshot.Status != "" {
				out.Probe = &snapshot
			}
		}
	}
	return out
}

func applyUpstreamEntity(in *service.Upstream, m *dbent.Upstream) {
	if in == nil || m == nil {
		return
	}
	in.ID = m.ID
	in.ConfigVersion = m.ConfigVersion
	in.Enabled = m.Enabled
	in.IntervalMinutes = m.IntervalMinutes
	in.CreatedAt = m.CreatedAt
	in.UpdatedAt = m.UpdatedAt
}
