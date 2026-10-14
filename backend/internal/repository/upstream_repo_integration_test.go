//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newTestUpstream(name string, proxyID *int64) *service.Upstream {
	return &service.Upstream{
		Name:            name,
		BaseURL:         "https://upstream.example",
		APIKeyEncrypted: "cipher-a",
		ProxyID:         proxyID,
		Enabled:         true,
		IntervalMinutes: 15,
		ConfigVersion:   "v1-" + name,
	}
}

func TestUpstreamRepositoryProbeResultCAS(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newUpstreamRepositoryWithSQL(tx.Client(), tx)
	txCtx := ctx

	u := newTestUpstream("cas", nil)
	require.NoError(t, repo.Create(txCtx, u))
	loaded, err := repo.GetByID(txCtx, u.ID)
	require.NoError(t, err)
	require.Nil(t, loaded.Probe)

	now := time.Now().UTC().Truncate(time.Second)
	remaining := 0.0
	snapshot := &service.UpstreamProbeSnapshot{
		Status: "ok",
		Data: &service.UpstreamUsageData{Unit: "USD", Items: map[string]*service.UpstreamUsageItem{
			"wallet": {Remaining: &remaining},
		}},
		LastAttemptAt: now,
		NextProbeAt:   now.Add(15 * time.Minute),
	}
	require.NoError(t, repo.UpdateProbeResult(txCtx, u.ID, loaded.ConfigVersion, nil, nil, snapshot))
	current, err := repo.GetByID(txCtx, u.ID)
	require.NoError(t, err)
	require.NotNil(t, current.Probe)
	require.Equal(t, 0.0, *current.Probe.Data.Items["wallet"].Remaining)
	require.NotNil(t, current.NextProbeAt)

	// 只改名称／开关／周期：不换版本，不清快照。
	changed, err := repo.Update(txCtx, service.UpstreamUpdate{
		ID: u.ID, Name: "renamed", BaseURL: current.BaseURL, ProxyID: nil,
		Enabled: false, IntervalMinutes: 30, NewConfigVersion: "v2",
	})
	require.NoError(t, err)
	require.False(t, changed)
	current, err = repo.GetByID(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, loaded.ConfigVersion, current.ConfigVersion)
	require.NotNil(t, current.Probe)
	require.Equal(t, "cipher-a", current.APIKeyEncrypted)

	// 首次请求无快照时轮换 Key（A→B）：迟到的旧结果必须被拒绝。
	rotated := "cipher-b"
	changed, err = repo.Update(txCtx, service.UpstreamUpdate{
		ID: u.ID, Name: "renamed", BaseURL: current.BaseURL, APIKeyEncrypted: &rotated,
		Enabled: true, IntervalMinutes: 30, NewConfigVersion: "v3",
	})
	require.NoError(t, err)
	require.True(t, changed)
	current, err = repo.GetByID(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "v3", current.ConfigVersion)
	require.Nil(t, current.Probe)
	require.Nil(t, current.NextProbeAt)
	require.Zero(t, current.FailureCount)
	err = repo.UpdateProbeResult(txCtx, u.ID, loaded.ConfigVersion, nil, nil, snapshot)
	require.ErrorIs(t, err, service.ErrUpstreamProbeIdentityChanged)

	// 地址改回原值（B→A）同样换版本：v3 期间发出的请求写回仍被拒绝。
	changed, err = repo.Update(txCtx, service.UpstreamUpdate{
		ID: u.ID, Name: "renamed", BaseURL: "https://other.example",
		Enabled: true, IntervalMinutes: 30, NewConfigVersion: "v4",
	})
	require.NoError(t, err)
	require.True(t, changed)
	err = repo.UpdateProbeResult(txCtx, u.ID, "v3", nil, nil, snapshot)
	require.ErrorIs(t, err, service.ErrUpstreamProbeIdentityChanged)
	require.NoError(t, repo.UpdateProbeResult(txCtx, u.ID, "v4", nil, nil, snapshot))

	// 删除后迟到写回被拒绝，快照被清理。
	require.NoError(t, repo.Delete(txCtx, u.ID))
	_, err = repo.GetByID(txCtx, u.ID)
	require.ErrorIs(t, err, service.ErrUpstreamNotFound)
	err = repo.UpdateProbeResult(txCtx, u.ID, "v4", nil, nil, snapshot)
	require.ErrorIs(t, err, service.ErrUpstreamProbeIdentityChanged)
	rows, err := tx.QueryContext(txCtx, `SELECT probe IS NULL FROM upstreams WHERE id = $1`, u.ID)
	require.NoError(t, err)
	require.True(t, rows.Next())
	var probeIsNull bool
	require.NoError(t, rows.Scan(&probeIsNull))
	require.NoError(t, rows.Close())
	require.True(t, probeIsNull)
	require.ErrorIs(t, repo.Delete(txCtx, u.ID), service.ErrUpstreamNotFound)
}

func TestUpstreamRepositoryProxyIdentityChange(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newUpstreamRepositoryWithSQL(tx.Client(), tx)
	proxyRepo := newProxyRepositoryWithSQL(tx.Client(), tx)
	proxy := &service.Proxy{Name: "上游代理", Protocol: "http", Host: "127.0.0.1", Port: 18001, Username: "Default.{account_id}", Password: "token", Status: service.StatusActive}
	require.NoError(t, proxyRepo.Create(ctx, proxy))
	used, err := proxyRepo.GetByID(ctx, proxy.ID)
	require.NoError(t, err)

	u := newTestUpstream("proxy", &proxy.ID)
	require.NoError(t, repo.Create(ctx, u))
	snapshot := &service.UpstreamProbeSnapshot{Status: "ok", LastAttemptAt: time.Now(), NextProbeAt: time.Now().Add(time.Hour)}

	// 未渲染的模板原文与探测时一致即通过。
	require.NoError(t, repo.UpdateProbeResult(ctx, u.ID, u.ConfigVersion, &proxy.ID, used, snapshot))

	// proxy_id 不变但代理主机被改：迟到结果拒绝。
	_, err = tx.ExecContext(ctx, `UPDATE proxies SET host = '127.0.0.2' WHERE id = $1`, proxy.ID)
	require.NoError(t, err)
	err = repo.UpdateProbeResult(ctx, u.ID, u.ConfigVersion, &proxy.ID, used, snapshot)
	require.ErrorIs(t, err, service.ErrUpstreamProbeIdentityChanged)

	// 探测开始时代理已不可用（proxy=nil），写回时仍不存在：允许记录 proxy_unavailable。
	_, err = tx.ExecContext(ctx, `UPDATE proxies SET deleted_at = NOW() WHERE id = $1`, proxy.ID)
	require.NoError(t, err)
	failed := &service.UpstreamProbeSnapshot{Status: "failed", LastError: "proxy_unavailable", FailureCount: 1, LastAttemptAt: time.Now(), NextProbeAt: time.Now().Add(time.Hour)}
	require.NoError(t, repo.UpdateProbeResult(ctx, u.ID, u.ConfigVersion, &proxy.ID, nil, failed))
	current, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "proxy_unavailable", current.LastError)
	require.Equal(t, 1, current.FailureCount)
}

func TestUpstreamRepositoryListDueOrderingAndFilters(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newUpstreamRepositoryWithSQL(tx.Client(), tx)
	now := time.Now().UTC().Truncate(time.Second)

	create := func(name string, enabled bool, next *time.Time) *service.Upstream {
		u := newTestUpstream(name, nil)
		u.Enabled = enabled
		require.NoError(t, repo.Create(ctx, u))
		if next != nil {
			_, err := tx.ExecContext(ctx, `UPDATE upstreams SET next_probe_at = $2 WHERE id = $1`, u.ID, *next)
			require.NoError(t, err)
		}
		return u
	}
	past := now.Add(-time.Hour)
	earlier := now.Add(-2 * time.Hour)
	future := now.Add(time.Hour)
	dueOld := create("due-old", true, &earlier)
	dueNull := create("due-null", true, nil)
	dueRecent := create("due-recent", true, &past)
	create("future", true, &future)
	create("disabled", false, nil)
	deleted := create("deleted", true, nil)
	require.NoError(t, repo.Delete(ctx, deleted.ID))

	due, err := repo.ListDue(ctx, now, 10)
	require.NoError(t, err)
	ids := make([]int64, 0, len(due))
	for _, item := range due {
		ids = append(ids, item.ID)
	}
	// NULL 最先，其余按 next_probe_at 升序；未到期、停用和已删除都不出现。
	require.Equal(t, []int64{dueNull.ID, dueOld.ID, dueRecent.ID}, ids)

	limited, err := repo.ListDue(ctx, now, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	require.Equal(t, dueNull.ID, limited[0].ID)

	items, result, err := repo.List(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, "due-")
	require.NoError(t, err)
	require.Equal(t, int64(3), result.Total)
	require.Len(t, items, 3)
}

func TestUpstreamRepositoryListSearchesNameAndBaseURL(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newUpstreamRepositoryWithSQL(tx.Client(), tx)

	byName := newTestUpstream("主站上游", nil)
	byURL := newTestUpstream("备用", nil)
	byURL.BaseURL = "https://relay.example.net"
	require.NoError(t, repo.Create(ctx, byName))
	require.NoError(t, repo.Create(ctx, byURL))

	params := pagination.PaginationParams{Page: 1, PageSize: 20}
	items, _, err := repo.List(ctx, params, "RELAY.example")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, byURL.ID, items[0].ID)

	items, _, err = repo.List(ctx, params, "主站")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, byName.ID, items[0].ID)
}
