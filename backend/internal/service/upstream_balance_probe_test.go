//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"

	"github.com/stretchr/testify/require"
)

func TestParseUpstreamBalance(t *testing.T) {
	for _, tc := range []struct {
		name, kind, body, reason string
		remaining                float64
		unlimited                bool
	}{
		{name: "钱包零余额", kind: "wallet", body: `{"mode":"unrestricted","balance":0,"unit":"USD"}`},
		{name: "钱包欠费", kind: "wallet", body: `{"mode":"unrestricted","balance":-1}`, remaining: -1},
		{name: "不拿根剩余替代钱包", kind: "wallet", body: `{"mode":"unrestricted","remaining":10}`, reason: "unavailable"},
		{name: "额度模式不含钱包", kind: "wallet", body: `{"mode":"quota_limited","remaining":20}`, reason: "unavailable"},
		{name: "Key总额度", kind: "key_total", body: `{"mode":"quota_limited","quota":{"limit":20,"used":7,"remaining":13}}`, remaining: 13},
		{name: "Key周期选定窗口", kind: "key_1d", body: `{"mode":"quota_limited","rate_limits":[{"window":"5h","limit":10,"used":2,"remaining":8},{"window":"1d","limit":20,"used":3,"remaining":17}]}`, remaining: 17},
		{name: "未配置的窗口不可用", kind: "key_7d", body: `{"mode":"quota_limited","rate_limits":[]}`, reason: "unavailable"},
		{name: "订阅日额度", kind: "subscription_daily", body: `{"mode":"unrestricted","subscription":{"daily_limit_usd":12,"daily_usage_usd":4}}`, remaining: 8},
		{name: "订阅超额归零", kind: "subscription_weekly", body: `{"mode":"unrestricted","subscription":{"weekly_limit_usd":12,"weekly_usage_usd":14}}`},
		{name: "明确无周期限额", kind: "subscription_monthly", body: `{"mode":"unrestricted","subscription":{"monthly_limit_usd":null,"monthly_usage_usd":3}}`, unlimited: true},
		{name: "缺字段不是无限制", kind: "subscription_monthly", body: `{"mode":"unrestricted","subscription":{"monthly_usage_usd":3}}`, reason: "unavailable"},
		{name: "综合无限制", kind: "subscription_remaining", body: `{"mode":"unrestricted","remaining":-1,"subscription":{"daily_usage_usd":0}}`, unlimited: true},
		{name: "错误的负综合额度", kind: "subscription_remaining", body: `{"mode":"unrestricted","remaining":-2,"subscription":{"daily_usage_usd":0}}`, reason: "invalid_response"},
		{name: "非数值", kind: "wallet", body: `{"mode":"unrestricted","balance":"12"}`, reason: "invalid_response"},
		{name: "非有限值", kind: "wallet", body: `{"mode":"unrestricted","balance":1e999}`, reason: "invalid_response"},
		{name: "错误单位", kind: "wallet", body: `{"mode":"unrestricted","balance":12,"unit":"CNY"}`, reason: "invalid_response"},
		{name: "未知协议", kind: "wallet", body: `{"balance":12}`, reason: "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, reason := parseUpstreamBalance([]byte(tc.body), tc.kind)
			require.Equal(t, tc.reason, reason)
			if reason != "" {
				require.Nil(t, data)
				return
			}
			require.Equal(t, "USD", data.Unit)
			require.Equal(t, tc.unlimited, data.Unlimited)
			if !tc.unlimited {
				require.NotNil(t, data.Remaining)
				require.Equal(t, tc.remaining, *data.Remaining)
			}
		})
	}
}

func TestUpstreamBalanceConfigValidation(t *testing.T) {
	for _, value := range []any{nil, 1, true, "", "automatic", map[string]any{"wallet": true}} {
		require.Error(t, ValidateUpstreamBalanceProbeType(value))
	}
	_, err := buildAccountForCreate(&CreateAccountInput{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, map[string]any{UpstreamBalanceProbeTypeExtraKey: "wallet"})
	require.ErrorIs(t, err, ErrUpstreamBillingProbeAccountInvalid)
	account, err := buildAccountForCreate(&CreateAccountInput{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, map[string]any{
		UpstreamBalanceProbeTypeExtraKey: "wallet", UpstreamBalanceProbeExtraKey: map[string]any{"status": "ok"},
	})
	require.NoError(t, err)
	require.Equal(t, "wallet", UpstreamBalanceProbeType(account))
	require.NotContains(t, account.Extra, UpstreamBalanceProbeExtraKey)
}

// 复用原探测夹具，确保共享传输仍带账号鉴权，余额不写倍率字段。
type balanceProbeRepoStub struct {
	*upstreamBillingProbeAccountRepo
	balance *UpstreamBalanceProbeSnapshot
}

func (r *balanceProbeRepoStub) UpdateUpstreamBalanceProbeSnapshot(_ context.Context, account *Account, snapshot *UpstreamBalanceProbeSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.balance = snapshot
	r.accounts[account.ID].Extra[UpstreamBalanceProbeExtraKey] = snapshot
	return nil
}

func (r *balanceProbeRepoStub) ListDueUpstreamBillingProbeAccounts(_ context.Context, _ time.Time, limit int) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	accounts := make([]Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		accounts = append(accounts, *account)
		if len(accounts) == limit {
			break
		}
	}
	return accounts, nil
}

type balanceProbeHTTPStub struct {
	HTTPUpstream
	request func(*http.Request, string) *http.Response
}

func (h *balanceProbeHTTPStub) DoWithTLS(req *http.Request, proxyURL string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return h.request(req, proxyURL), nil
}

func TestUpstreamBalanceManualRequestAndFailureBackoff(t *testing.T) {
	now := time.Now().UTC()
	initialRate := 0.5
	proxyID := int64(7)
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: &initialRate,
		ProxyID: &proxyID, Credentials: map[string]any{"api_key": "sk-local-test", "base_url": "https://upstream.example/v1"},
		Extra: map[string]any{UpstreamBalanceProbeTypeExtraKey: "wallet"},
	}
	account.Proxy = (&Proxy{ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 8080, Username: "session.{account_id}", Password: "test"}).ForAccount(account)
	repo := &balanceProbeRepoStub{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{42: account}}}
	status, body := 200, `{"mode":"unrestricted","balance":12}`
	httpStub := &balanceProbeHTTPStub{request: func(req *http.Request, proxyURL string) *http.Response {
		require.Equal(t, "https://upstream.example/v1/usage", req.URL.String())
		require.Equal(t, "Bearer sk-local-test", req.Header.Get("Authorization"))
		require.Contains(t, proxyURL, "session.42")
		require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
		_, hasDeadline := req.Context().Deadline()
		require.True(t, hasDeadline)
		header := http.Header{}
		if status == 429 {
			header.Set("Retry-After", "7200")
		}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
	}}
	settingRepo := &upstreamBillingProbeSettingRepo{values: map[string]string{SettingKeyUpstreamBillingProbeSettings: `{"enabled":false,"interval_minutes":30}`}}
	s := newUpstreamBillingProbeTestService(repo, httpStub, settingRepo)
	s.now = func() time.Time { return now }
	result, err := s.ProbeBalanceAccount(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, "ok", result.Status)
	require.Equal(t, 12.0, *result.Data.Remaining)
	require.Equal(t, 0.5, account.BillingRateMultiplier())
	status, body = 429, `{"error":"secret upstream error must not be saved"}`
	failed, err := s.ProbeBalanceAccount(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	require.Equal(t, "http_error", failed.LastError)
	require.Equal(t, result.Data, failed.Data)
	require.Equal(t, now.Add(2*time.Hour), failed.NextProbeAt)
	status = 503
	_, err = s.ProbeBalanceAccount(context.Background(), account.ID)
	require.NoError(t, err)
	failed, err = s.ProbeBalanceAccount(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, 3, failed.FailureCount)
	require.GreaterOrEqual(t, failed.NextProbeAt.Sub(now), 115*time.Minute)
	status, body = 200, `{"mode":"quota_limited","quota":{"limit":10,"used":1,"remaining":9}}`
	unavailable, err := s.ProbeBalanceAccount(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, "unavailable", unavailable.Status)
	require.Nil(t, unavailable.Data)
}

func TestUpstreamBalanceSnapshotTypeAndDue(t *testing.T) {
	now := time.Now().UTC()
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
		UpstreamBalanceProbeTypeExtraKey: "wallet",
		UpstreamBalanceProbeExtraKey:     map[string]any{"type": "key_total", "status": "ok", "next_probe_at": now.Add(time.Hour).Format(time.RFC3339Nano)},
	}}
	require.Nil(t, DecodeUpstreamBalanceProbeSnapshot(account))
	next, enabled := nextUpstreamAccountProbe(account)
	require.True(t, enabled)
	require.True(t, next.IsZero())
	account.Extra[UpstreamBalanceProbeTypeExtraKey] = "disabled"
	_, enabled = nextUpstreamAccountProbe(account)
	require.False(t, enabled)
}

func TestUpstreamBalanceScheduledProbeIsIndependent(t *testing.T) {
	for _, billingEnabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "仅启用余额", true: "倍率仍在退避"}[billingEnabled], func(t *testing.T) {
			now := time.Now().UTC()
			account := &Account{
				ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
				Credentials: map[string]any{"api_key": "sk-local-test", "base_url": "https://upstream.example"},
				Extra: map[string]any{
					UpstreamBalanceProbeTypeExtraKey: "wallet", UpstreamBillingProbeEnabledExtraKey: billingEnabled,
					UpstreamBillingProbeExtraKey: &UpstreamBillingProbeSnapshot{Status: UpstreamBillingProbeStatusUnsupported, NextProbeAt: now.Add(time.Hour)},
				},
			}
			repo := &balanceProbeRepoStub{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{42: account}}}
			calls := 0
			upstream := &balanceProbeHTTPStub{request: func(req *http.Request, _ string) *http.Response {
				calls++
				require.Equal(t, "/v1/usage", req.URL.Path)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"mode":"unrestricted","balance":12}`))}
			}}
			settings := &upstreamBillingProbeSettingRepo{values: map[string]string{SettingKeyUpstreamBillingProbeSettings: `{"enabled":true,"interval_minutes":30}`}}
			svc := newUpstreamBillingProbeTestService(repo, upstream, settings)
			svc.now = func() time.Time { return now }
			require.NoError(t, svc.RunDue(context.Background()))
			require.Equal(t, 1, calls)
			require.Equal(t, 12.0, *repo.balance.Data.Remaining)
			require.NoError(t, svc.RunDue(context.Background()))
			require.Equal(t, 1, calls, "未到期不能重复请求")
			now = now.Add(2 * time.Hour)
			require.NoError(t, settings.Set(context.Background(), SettingKeyUpstreamBillingProbeSettings, `{"enabled":false,"interval_minutes":30}`))
			require.NoError(t, svc.RunDue(context.Background()))
			require.Equal(t, 1, calls, "全局关闭同时暂停余额定时请求")
		})
	}
}
