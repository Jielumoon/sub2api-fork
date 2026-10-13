//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUpstreamBalanceProbePersistence(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	initialRate := 1.0
	account := &service.Account{
		Name: "余额探测", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Concurrency: 1, RateMultiplier: &initialRate,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://upstream.example"},
		Extra:       map[string]any{service.UpstreamBalanceProbeTypeExtraKey: "wallet", "custom": "保留"},
	}
	require.NoError(t, repo.Create(ctx, account))
	loaded, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	amount := 0.0
	snapshot := &service.UpstreamBalanceProbeSnapshot{
		Type: "wallet", Status: "ok", Data: &service.UpstreamBalanceProbeData{Remaining: &amount, Unit: "USD"},
		LastAttemptAt: time.Now(), NextProbeAt: time.Now().Add(time.Hour),
	}
	// 业务更新不修改探测配置版本，不能因为 updated_at 改变而拒绝余额。
	require.NoError(t, repo.UpdateExtra(ctx, account.ID, map[string]any{"other": true}))
	require.NoError(t, repo.UpdateUpstreamBalanceProbeSnapshot(ctx, loaded, snapshot))
	current, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "保留", current.Extra["custom"])
	require.Equal(t, true, current.Extra["other"])
	require.Equal(t, 0.0, *service.DecodeUpstreamBalanceProbeSnapshot(current).Data.Remaining)
	// 普通编辑保留数据库最新快照，忽略调用方携带的旧快照。
	loaded.Name = "普通编辑"
	require.NoError(t, repo.Update(ctx, loaded))
	current, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, service.DecodeUpstreamBalanceProbeSnapshot(current))
	// 两类观测只更新自己的槽，余额不修改倍率。
	require.NoError(t, repo.UpdateExtra(ctx, account.ID, map[string]any{service.UpstreamBillingProbeEnabledExtraKey: true}))
	current, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	rate := &service.UpstreamBillingProbeSnapshot{Status: "ok", NextProbeAt: time.Now().Add(time.Hour)}
	require.NoError(t, repo.UpdateUpstreamBillingProbeSnapshot(ctx, current, rate, nil))
	current, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, service.DecodeUpstreamBalanceProbeSnapshot(current))
	require.Contains(t, current.Extra, service.UpstreamBillingProbeExtraKey)
	require.Equal(t, 1.0, current.BillingRateMultiplier())
}

func TestUpstreamBalanceProbeRejectsFirstRequestAfterConfigRoundTrip(t *testing.T) {
	for _, change := range []string{"type", "credentials", "bulk", "proxy"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			tx := testEntTx(t)
			repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
			initialRate := 1.0
			proxyRepo := newProxyRepositoryWithSQL(tx.Client(), tx)
			proxy := &service.Proxy{Name: "余额代理", Protocol: "http", Host: "127.0.0.1", Port: 18000, Status: service.StatusActive}
			require.NoError(t, proxyRepo.Create(ctx, proxy))
			account := &service.Account{
				Name: "首次余额请求", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Concurrency: 1, RateMultiplier: &initialRate, ProxyID: &proxy.ID,
				Credentials: map[string]any{"api_key": "sk-a"},
				Extra:       map[string]any{service.UpstreamBalanceProbeTypeExtraKey: "wallet"},
			}
			require.NoError(t, repo.Create(ctx, account))
			loaded, err := repo.GetByID(ctx, account.ID)
			require.NoError(t, err)
			switch change {
			case "type":
				for _, kind := range []string{"key_total", "wallet"} {
					current, err := repo.GetByID(ctx, account.ID)
					require.NoError(t, err)
					require.NoError(t, repo.UpdateWithAccountBillingSettings(ctx, current, nil, nil, nil, &kind))
				}
			case "credentials":
				require.NoError(t, repo.UpdateCredentials(ctx, account.ID, map[string]any{"api_key": "sk-b"}))
				require.NoError(t, repo.UpdateCredentials(ctx, account.ID, loaded.Credentials))
			case "bulk":
				for _, kind := range []string{"key_total", "wallet"} {
					_, err := repo.BulkUpdate(ctx, []int64{account.ID}, service.AccountBulkUpdate{Extra: map[string]any{service.UpstreamBalanceProbeTypeExtraKey: kind}})
					require.NoError(t, err)
				}
			case "proxy":
				proxy.Port = 18001
				require.NoError(t, proxyRepo.Update(ctx, proxy))
				proxy.Port = 18000
				require.NoError(t, proxyRepo.Update(ctx, proxy))
			}
			snapshot := &service.UpstreamBalanceProbeSnapshot{Type: "wallet", Status: "ok", NextProbeAt: time.Now().Add(time.Hour)}
			require.ErrorIs(t, repo.UpdateUpstreamBalanceProbeSnapshot(ctx, loaded, snapshot), service.ErrUpstreamBillingProbeIdentityChanged)
			current, err := repo.GetByID(ctx, account.ID)
			require.NoError(t, err)
			require.NotContains(t, current.Extra, service.UpstreamBalanceProbeExtraKey)
			require.Equal(t, "wallet", service.UpstreamBalanceProbeType(current))
		})
	}
}

func TestUpstreamBalanceProbeDueWithoutBilling(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	initialRate := 1.0
	account := &service.Account{
		Name: "只探测余额", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Concurrency: 1, RateMultiplier: &initialRate,
		Credentials: map[string]any{"api_key": "sk-test"},
		Extra:       map[string]any{service.UpstreamBalanceProbeTypeExtraKey: "wallet"},
	}
	require.NoError(t, repo.Create(ctx, account))
	due, err := repo.ListDueUpstreamBillingProbeAccounts(ctx, time.Now().Add(time.Second), 20)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, account.ID, due[0].ID)
	snapshot := &service.UpstreamBalanceProbeSnapshot{Type: "wallet", Status: "ok", NextProbeAt: time.Now().Add(time.Hour)}
	loaded, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateUpstreamBalanceProbeSnapshot(ctx, loaded, snapshot))
	due, err = repo.ListDueUpstreamBillingProbeAccounts(ctx, time.Now(), 20)
	require.NoError(t, err)
	require.Empty(t, due)
}
