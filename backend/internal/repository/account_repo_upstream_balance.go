package repository

import (
	"context"
	"encoding/json"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) UpdateUpstreamBalanceProbeSnapshot(ctx context.Context, account *service.Account, snapshot *service.UpstreamBalanceProbeSnapshot) error {
	if account == nil || snapshot == nil {
		return service.ErrAccountNilInput
	}
	if dbent.TxFromContext(ctx) != nil {
		return r.updateUpstreamBalanceProbeSnapshotInTx(ctx, account, snapshot)
	}
	tx, err := r.client.Tx(ctx)
	if errors.Is(err, dbent.ErrTxStarted) {
		return r.updateUpstreamBalanceProbeSnapshotInTx(ctx, account, snapshot)
	}
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.updateUpstreamBalanceProbeSnapshotInTx(dbent.NewTxContext(ctx, tx), account, snapshot); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *accountRepository) updateUpstreamBalanceProbeSnapshotInTx(ctx context.Context, account *service.Account, snapshot *service.UpstreamBalanceProbeSnapshot) error {
	client := clientFromContext(ctx, r.client)
	matches, err := lockAndMatchProbeProxyIdentity(ctx, client, account)
	if err != nil {
		return err
	}
	if !matches {
		return service.ErrUpstreamBillingProbeIdentityChanged
	}
	payload, err := json.Marshal(map[string]any{service.UpstreamBalanceProbeExtraKey: snapshot})
	if err != nil {
		return err
	}
	credentials, err := json.Marshal(account.Credentials)
	if err != nil {
		return err
	}
	expected, err := json.Marshal(account.Extra[service.UpstreamBalanceProbeExtraKey])
	if err != nil {
		return err
	}
	revision, err := json.Marshal(account.Extra[service.UpstreamBalanceProbeRevisionExtraKey])
	if err != nil {
		return err
	}
	var proxyID any
	if account.ProxyID != nil {
		proxyID = *account.ProxyID
	}
	// 配置版本拒绝 A→B→A 的迟到结果；业务请求更新 updated_at 不会打断探测。
	result, err := client.ExecContext(ctx, `
		UPDATE accounts SET extra = COALESCE(extra, '{}'::jsonb) || $1::jsonb
		WHERE id = $2 AND deleted_at IS NULL
			AND platform = $3 AND type = $4 AND credentials = $5::jsonb
			AND proxy_id IS NOT DISTINCT FROM $6
			AND COALESCE(extra -> 'upstream_balance_probe_revision', 'null'::jsonb) = $7::jsonb
			AND extra ->> 'upstream_balance_probe_type' = $8
			AND COALESCE(extra -> 'upstream_balance_probe', 'null'::jsonb) = $9::jsonb
	`, string(payload), account.ID, account.Platform, account.Type, string(credentials), proxyID, string(revision), snapshot.Type, string(expected))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrUpstreamBillingProbeIdentityChanged
	}
	return nil
}
