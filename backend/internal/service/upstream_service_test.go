//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"

	"github.com/stretchr/testify/require"
)

// ---------- 解析 ----------

func TestParseUpstreamUsageAll(t *testing.T) {
	for _, tc := range []struct {
		name, body, reason string
		items              []string
		unlimited          string
		expiresAt          string
	}{
		{
			name:  "只返回钱包时只收集钱包",
			body:  `{"mode":"unrestricted","balance":12.5,"unit":"USD","status":"active"}`,
			items: []string{"wallet"},
		},
		{
			name: "Key 模式收集总额度与三个窗口",
			body: `{"mode":"quota_limited","quota":{"limit":20,"used":7,"remaining":13},"rate_limits":[
				{"window":"5h","limit":5,"used":1,"remaining":4},
				{"window":"1d","limit":10,"used":2,"remaining":8},
				{"window":"7d","limit":50,"used":9,"remaining":41}]}`,
			items: []string{"key_total", "key_5h", "key_1d", "key_7d"},
		},
		{
			name:      "订阅模式收集周期与综合额度",
			body:      `{"mode":"unrestricted","remaining":6,"subscription":{"daily_limit_usd":10,"daily_usage_usd":4,"weekly_limit_usd":50,"weekly_usage_usd":10,"monthly_limit_usd":100,"monthly_usage_usd":20,"expires_at":"2026-01-02T03:04:05Z"}}`,
			items:     []string{"subscription_daily", "subscription_weekly", "subscription_monthly", "subscription_remaining"},
			expiresAt: "2026-01-02T03:04:05Z",
		},
		{
			name:      "明确 null 限额为无限制",
			body:      `{"mode":"unrestricted","remaining":-1,"subscription":{"daily_limit_usd":null,"daily_usage_usd":3,"weekly_limit_usd":0,"weekly_usage_usd":1}}`,
			items:     []string{"subscription_daily", "subscription_weekly", "subscription_remaining"},
			unlimited: "subscription_daily",
		},
		{
			name:   "缺失字段不是零",
			body:   `{"mode":"unrestricted","subscription":{"daily_usage_usd":3}}`,
			reason: "unavailable",
		},
		{
			name:  "未提供单位也接受",
			body:  `{"mode":"unrestricted","balance":0}`,
			items: []string{"wallet"},
		},
		{
			name:   "未知协议",
			body:   `{"balance":12}`,
			reason: "unsupported",
		},
		{
			name:   "非法单位",
			body:   `{"mode":"unrestricted","balance":12,"unit":"CNY"}`,
			reason: "invalid_response",
		},
		{
			name:   "非有限数值",
			body:   `{"mode":"unrestricted","balance":1e999}`,
			reason: "invalid_response",
		},
		{
			name:   "错误的负综合额度",
			body:   `{"mode":"unrestricted","remaining":-2,"subscription":{"daily_limit_usd":10,"daily_usage_usd":1}}`,
			reason: "invalid_response",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, reason := parseUpstreamUsageAll([]byte(tc.body))
			require.Equal(t, tc.reason, reason)
			if tc.reason != "" {
				require.Nil(t, data)
				return
			}
			require.Equal(t, "USD", data.Unit)
			require.Len(t, data.Items, len(tc.items))
			for _, kind := range tc.items {
				require.Contains(t, data.Items, kind)
			}
			if tc.unlimited != "" {
				require.True(t, data.Items[tc.unlimited].Unlimited)
				require.Nil(t, data.Items[tc.unlimited].Remaining)
			}
			require.Equal(t, tc.expiresAt, data.ExpiresAt)
		})
	}
}

func TestParseUpstreamUsageAllKeepsWalletZero(t *testing.T) {
	data, reason := parseUpstreamUsageAll([]byte(`{"mode":"unrestricted","balance":0}`))
	require.Empty(t, reason)
	require.NotNil(t, data.Items["wallet"].Remaining)
	require.Equal(t, 0.0, *data.Items["wallet"].Remaining)
}

// ---------- 夹具 ----------

type upstreamRepoStub struct {
	items       map[int64]*Upstream
	listErr     error
	updated     []*UpstreamProbeSnapshot
	lastProxy   *Proxy
	casMismatch bool
}

func (r *upstreamRepoStub) Create(_ context.Context, in *Upstream) error {
	if in.ID == 0 {
		in.ID = int64(len(r.items) + 1)
	}
	if in.ConfigVersion == "" {
		in.ConfigVersion = "v1"
	}
	copied := *in
	r.items[in.ID] = &copied
	return nil
}

func (r *upstreamRepoStub) GetByID(_ context.Context, id int64) (*Upstream, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, ErrUpstreamNotFound
	}
	copied := *item
	return &copied, nil
}

func (r *upstreamRepoStub) List(_ context.Context, _ pagination.PaginationParams, _ string) ([]Upstream, *pagination.PaginationResult, error) {
	out := make([]Upstream, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, *item)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *upstreamRepoStub) Update(_ context.Context, in UpstreamUpdate) (bool, error) {
	item, ok := r.items[in.ID]
	if !ok {
		return false, ErrUpstreamNotFound
	}
	changed := item.BaseURL != in.BaseURL || item.ProxyID != nil && in.ProxyID == nil
	if item.ProxyID != nil && in.ProxyID != nil && *item.ProxyID != *in.ProxyID {
		changed = true
	}
	if item.ProxyID == nil && in.ProxyID != nil {
		changed = true
	}
	if in.APIKeyEncrypted != nil && *in.APIKeyEncrypted != item.APIKeyEncrypted {
		changed = true
	}
	item.Name, item.BaseURL, item.Enabled, item.IntervalMinutes = in.Name, in.BaseURL, in.Enabled, in.IntervalMinutes
	item.ProxyID = in.ProxyID
	if in.APIKeyEncrypted != nil {
		item.APIKeyEncrypted = *in.APIKeyEncrypted
	}
	if changed {
		item.ConfigVersion = in.NewConfigVersion
		item.Probe = nil
		item.NextProbeAt = nil
		item.FailureCount = 0
	}
	return changed, nil
}

func (r *upstreamRepoStub) Delete(_ context.Context, id int64) error {
	if _, ok := r.items[id]; !ok {
		return ErrUpstreamNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *upstreamRepoStub) ListDue(_ context.Context, _ time.Time, _ int) ([]Upstream, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]Upstream, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, *item)
	}
	return out, nil
}

func (r *upstreamRepoStub) UpdateProbeResult(_ context.Context, id int64, configVersion string, _ *int64, proxy *Proxy, snapshot *UpstreamProbeSnapshot) error {
	if r.casMismatch {
		return ErrUpstreamProbeIdentityChanged
	}
	item, ok := r.items[id]
	if !ok || item.ConfigVersion != configVersion {
		return ErrUpstreamProbeIdentityChanged
	}
	item.Probe = snapshot
	item.FailureCount = snapshot.FailureCount
	item.LastError = snapshot.LastError
	next := snapshot.NextProbeAt
	item.NextProbeAt = &next
	r.updated = append(r.updated, snapshot)
	r.lastProxy = proxy
	return nil
}

type upstreamHTTPStub struct {
	HTTPUpstream
	request   func(*http.Request, string) *http.Response
	lastURL   string
	lastKey   string
	lastProxy string
}

func (h *upstreamHTTPStub) DoWithTLS(req *http.Request, proxyURL string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	h.lastURL = req.URL.String()
	h.lastKey = req.Header.Get("Authorization")
	h.lastProxy = proxyURL
	return h.request(req, proxyURL), nil
}

type upstreamEncryptorStub struct {
	plain string
}

func (e *upstreamEncryptorStub) Encrypt(plaintext string) (string, error) {
	e.plain = plaintext
	return "enc:" + plaintext, nil
}

func (e *upstreamEncryptorStub) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, "enc:") {
		return "", errors.New("not encrypted by this stub")
	}
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}

func newUpstreamTestService(repo *upstreamRepoStub, upstream HTTPUpstream, proxyRepo ProxyRepository) (*UpstreamService, *upstreamEncryptorStub) {
	cfg := &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
		Enabled:           false,
		AllowInsecureHTTP: true,
	}}}
	accountTestService := &AccountTestService{httpUpstream: upstream, cfg: cfg}
	encryptor := &upstreamEncryptorStub{}
	svc := NewUpstreamService(repo, proxyRepo, accountTestService, encryptor)
	svc.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	svc.probeAsync = func(int64) {}
	return svc, encryptor
}

type upstreamProxyRepoStub struct {
	proxies   map[int64]*Proxy
	getCalled bool
}

func (r *upstreamProxyRepoStub) Create(context.Context, *Proxy) error { return nil }

func (r *upstreamProxyRepoStub) GetByID(_ context.Context, id int64) (*Proxy, error) {
	r.getCalled = true
	item, ok := r.proxies[id]
	if !ok {
		return nil, ErrProxyNotFound
	}
	return item, nil
}

func (r *upstreamProxyRepoStub) ListByIDs(context.Context, []int64) ([]Proxy, error) { return nil, nil }
func (r *upstreamProxyRepoStub) Update(context.Context, *Proxy) error                { return nil }
func (r *upstreamProxyRepoStub) Delete(context.Context, int64) error                 { return nil }

func (r *upstreamProxyRepoStub) List(context.Context, pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *upstreamProxyRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]Proxy, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *upstreamProxyRepoStub) ListWithFiltersAndAccountCount(context.Context, pagination.PaginationParams, string, string, string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *upstreamProxyRepoStub) ListActive(context.Context) ([]Proxy, error) { return nil, nil }

func (r *upstreamProxyRepoStub) ListActiveWithAccountCount(context.Context) ([]ProxyWithAccountCount, error) {
	return nil, nil
}

func (r *upstreamProxyRepoStub) ExistsByHostPortAuth(context.Context, string, int, string, string) (bool, error) {
	return false, nil
}

func (r *upstreamProxyRepoStub) CountAccountsByProxyID(context.Context, int64) (int64, error) {
	return 0, nil
}

func (r *upstreamProxyRepoStub) ListAccountSummariesByProxyID(context.Context, int64) ([]ProxyAccountSummary, error) {
	return nil, nil
}

func (r *upstreamProxyRepoStub) SweepExpiredProxies(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *upstreamProxyRepoStub) ListAllForFallback(context.Context) ([]Proxy, error) { return nil, nil }
func (r *upstreamProxyRepoStub) CountExpired(context.Context) (int64, error)         { return 0, nil }

func (r *upstreamProxyRepoStub) CountExpiringSoon(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func upstreamJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// ---------- 探测 ----------

func TestUpstreamServiceProbesAllQuotaItems(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(200, `{"mode":"quota_limited","quota":{"limit":20,"used":7,"remaining":13},"rate_limits":[{"window":"1d","limit":10,"used":2,"remaining":8}]}`)
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)

	created, err := svc.Create(context.Background(), CreateUpstreamInput{
		Name: "主站", BaseURL: "https://upstream.example/", APIKey: "sk-upstream-secret", IntervalMinutes: upstreamIntPtr(30),
	})
	require.NoError(t, err)
	require.Equal(t, "sk-u***", created.APIKeyMasked)
	require.True(t, created.APIKeyConfigured())

	snapshot, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "ok", snapshot.Status)
	require.Equal(t, 30, repo.items[created.ID].IntervalMinutes)
	require.Equal(t, "https://upstream.example/v1/usage", httpStub.lastURL)
	require.Equal(t, "Bearer sk-upstream-secret", httpStub.lastKey)
	require.Len(t, snapshot.Data.Items, 2)
	require.Equal(t, 13.0, *snapshot.Data.Items["key_total"].Remaining)
	require.Equal(t, 8.0, *snapshot.Data.Items["key_1d"].Remaining)
	// fresh_until = now + 2 * interval。
	require.Equal(t, svc.now().Add(60*time.Minute), *snapshot.FreshUntil)
}

func TestUpstreamServiceUnavailableClearsPreviousAmount(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	body := `{"mode":"unrestricted","balance":9}`
	status := 200
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(status, body)
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a"})
	require.NoError(t, err)

	first, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, 9.0, *first.Data.Items["wallet"].Remaining)

	// 上游改为返回 Subscription 但没有周期字段：没有任何可展示的额度项，旧金额不能继续展示。
	body, status = `{"mode":"unrestricted","subscription":{"expires_at":"2026-01-02T03:04:05Z"}}`, 200
	second, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "unavailable", second.Status)
	require.Nil(t, second.Data)

	// 上游不再提供 /v1/usage（404）：同样不能保留旧金额。
	body, status = `{"mode":"unrestricted","balance":9}`, 200
	_, err = svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	body, status = `not found`, 404
	third, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "unsupported", third.Status)
	require.Nil(t, third.Data)
	require.Nil(t, third.ReceivedAt)
}

func TestUpstreamServiceKeepsPreviousAmountOnFailure(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	status := 200
	body := `{"mode":"unrestricted","balance":9}`
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(status, body)
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a"})
	require.NoError(t, err)

	first, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)

	status, body = 500, `{}`
	second, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", second.Status)
	require.Equal(t, "http_error", second.LastError)
	require.Equal(t, 1, second.FailureCount)
	// 传输失败保留上次成功值与接收时间，由 fresh_until 过期标记陈旧。
	require.Equal(t, first.ReceivedAt, second.ReceivedAt)
	require.Equal(t, 9.0, *second.Data.Items["wallet"].Remaining)
}

func TestUpstreamServiceBackoffAndRetryAfter(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		resp := upstreamJSONResponse(500, `{}`)
		resp.Header.Set("Retry-After", "600")
		return resp
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a"})
	require.NoError(t, err)

	snapshot, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	delay := snapshot.NextProbeAt.Sub(snapshot.LastAttemptAt)
	// 首次失败以单倍 interval 15 分钟为基准（±20% 抖动，上限 5 分钟）；Retry-After 更长时由它接管。
	require.Greater(t, delay, 10*time.Minute)
	require.LessOrEqual(t, delay, 20*time.Minute)

	// 连续失败按失败次数放大退避：第二次失败以 2 倍 interval 为基准（±20% 抖动，上限 5 分钟），
	// 明显大于单倍 interval；Retry-After 更长时才由它接管。
	again, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, 2, again.FailureCount)
	delay = again.NextProbeAt.Sub(again.LastAttemptAt)
	require.Greater(t, delay, 20*time.Minute)
	require.LessOrEqual(t, delay, 35*time.Minute)

	// 429 的 Retry-After 长于本地退避时以上游指示为准。
	httpStub.request = func(*http.Request, string) *http.Response {
		resp := upstreamJSONResponse(http.StatusTooManyRequests, `{}`)
		resp.Header.Set("Retry-After", "3600")
		return resp
	}
	repo.items[created.ID].FailureCount = 0
	repo.items[created.ID].IntervalMinutes = 5
	throttled, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, time.Hour, throttled.NextProbeAt.Sub(throttled.LastAttemptAt))
}

func TestUpstreamServiceUnsupportedUsesStretchedDelay(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(404, `not found`)
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a"})
	require.NoError(t, err)

	snapshot, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "unsupported", snapshot.Status)
	// interval 15 分钟 × 8 倍拉伸，超过单倍间隔。
	require.GreaterOrEqual(t, snapshot.NextProbeAt.Sub(snapshot.LastAttemptAt), 100*time.Minute)
}

func TestUpstreamServiceProxyUnavailableMarksFailure(t *testing.T) {
	proxyID := int64(7)
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(200, `{"mode":"unrestricted","balance":1}`)
	}}
	proxyRepo := &upstreamProxyRepoStub{proxies: map[int64]*Proxy{}}
	svc, _ := newUpstreamTestService(repo, httpStub, proxyRepo)
	// 代理在创建后失效／被删除：此时只能由探测标记 failed，而不是静默跳过。
	require.NoError(t, repo.Create(context.Background(), &Upstream{
		ID: 1, Name: "主站", BaseURL: "https://upstream.example", APIKeyEncrypted: "enc:sk-a",
		ProxyID: &proxyID, Enabled: true, IntervalMinutes: 15, ConfigVersion: "v1",
	}))

	snapshot, err := svc.ProbeUpstream(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "failed", snapshot.Status)
	require.Equal(t, "proxy_unavailable", snapshot.LastError)
}

func TestUpstreamServiceRendersProxyWithoutAccount(t *testing.T) {
	proxyID := int64(7)
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(200, `{"mode":"unrestricted","balance":1}`)
	}}
	proxyRepo := &upstreamProxyRepoStub{proxies: map[int64]*Proxy{proxyID: {
		ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 8080,
		Username: "session.{account_id}", Password: "token", Status: StatusActive,
	}}}
	svc, _ := newUpstreamTestService(repo, httpStub, proxyRepo)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{
		Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a", ProxyID: &proxyID,
	})
	require.NoError(t, err)

	_, err = svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	// 上游没有账号上下文：占位符渲染为空（Resin 走随机出口），不借用上游 ID。
	require.Equal(t, "http://session.:token@proxy.example:8080", httpStub.lastProxy)
}

// 提前返回（未发请求）的路径也必须带回真实代理记录，否则写回时代理身份核对会把
// 「代理存在、传入 nil」判成配置已变，结果永远写不进去，next_probe_at 也不前移。
func TestUpstreamServiceEarlyExitStillCarriesProxy(t *testing.T) {
	proxyID := int64(7)
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		t.Fatal("官方 API 地址不应发出请求")
		return nil
	}}
	proxyRepo := &upstreamProxyRepoStub{proxies: map[int64]*Proxy{proxyID: {
		ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 8080, Status: StatusActive,
	}}}
	svc, _ := newUpstreamTestService(repo, httpStub, proxyRepo)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{
		Name: "官方", BaseURL: "https://api.openai.com", APIKey: "sk-a", ProxyID: &proxyID,
	})
	require.NoError(t, err)

	snapshot, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "unsupported", snapshot.Status)
	require.NotNil(t, repo.lastProxy)
	require.Equal(t, proxyID, repo.lastProxy.ID)
}

func TestUpstreamServiceCreateEncryptsAndUpdateKeepsKey(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(200, `{"mode":"unrestricted","balance":1}`)
	}}
	svc, encryptor := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-secret-key"})
	require.NoError(t, err)
	require.Equal(t, "enc:sk-secret-key", repo.items[created.ID].APIKeyEncrypted)
	require.Equal(t, "sk-secret-key", encryptor.plain)

	// 空 Key 表示不轮换。
	name := "改名"
	updated, err := svc.Update(context.Background(), created.ID, UpdateUpstreamInput{Name: &name})
	require.NoError(t, err)
	require.Equal(t, "改名", updated.Name)
	require.Equal(t, "enc:sk-secret-key", repo.items[created.ID].APIKeyEncrypted)

	// 非空 Key 覆盖并换配置版本。
	empty := ""
	_, err = svc.Update(context.Background(), created.ID, UpdateUpstreamInput{APIKey: &empty})
	require.NoError(t, err)
	require.Equal(t, "enc:sk-secret-key", repo.items[created.ID].APIKeyEncrypted)

	newKey := "sk-rotated"
	_, err = svc.Update(context.Background(), created.ID, UpdateUpstreamInput{APIKey: &newKey})
	require.NoError(t, err)
	require.Equal(t, "enc:sk-rotated", repo.items[created.ID].APIKeyEncrypted)
}

func TestUpstreamServiceUpdateConfigChangeResetsSnapshot(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(200, `{"mode":"unrestricted","balance":5}`)
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a"})
	require.NoError(t, err)
	_, err = svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.NotNil(t, repo.items[created.ID].Probe)
	previousVersion := repo.items[created.ID].ConfigVersion

	// 只改开关：保留版本与快照。
	enabled := false
	_, err = svc.Update(context.Background(), created.ID, UpdateUpstreamInput{Enabled: &enabled})
	require.NoError(t, err)
	require.Equal(t, previousVersion, repo.items[created.ID].ConfigVersion)
	require.NotNil(t, repo.items[created.ID].Probe)

	// 改地址：换版本、清快照、failure_count 归零。
	baseURL := "https://other.example"
	_, err = svc.Update(context.Background(), created.ID, UpdateUpstreamInput{BaseURL: &baseURL})
	require.NoError(t, err)
	require.NotEqual(t, previousVersion, repo.items[created.ID].ConfigVersion)
	require.Nil(t, repo.items[created.ID].Probe)
	require.Nil(t, repo.items[created.ID].NextProbeAt)
	require.Zero(t, repo.items[created.ID].FailureCount)
}

func TestUpstreamServiceRejectsConfigChangedDuringProbe(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		// 请求在途期间管理端改了地址：写回必须被 CAS 拒绝。
		repo.casMismatch = true
		return upstreamJSONResponse(200, `{"mode":"unrestricted","balance":5}`)
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a"})
	require.NoError(t, err)

	_, err = svc.ProbeUpstream(context.Background(), created.ID)
	require.ErrorIs(t, err, ErrUpstreamProbeIdentityChanged)
}

func TestUpstreamServiceValidation(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	svc, _ := newUpstreamTestService(repo, &upstreamHTTPStub{}, nil)
	ctx := context.Background()

	_, err := svc.Create(ctx, CreateUpstreamInput{Name: "  ", BaseURL: "https://a.example", APIKey: "sk"})
	require.ErrorIs(t, err, ErrUpstreamInvalidName)
	_, err = svc.Create(ctx, CreateUpstreamInput{Name: "主站", BaseURL: "not-a-url", APIKey: "sk"})
	require.ErrorIs(t, err, ErrUpstreamInvalidBaseURL)
	_, err = svc.Create(ctx, CreateUpstreamInput{Name: "主站", BaseURL: "https://a.example", APIKey: ""})
	require.ErrorIs(t, err, ErrUpstreamInvalidAPIKey)
	_, err = svc.Create(ctx, CreateUpstreamInput{Name: "主站", BaseURL: "https://a.example", APIKey: "sk", IntervalMinutes: upstreamIntPtr(4)})
	require.ErrorIs(t, err, ErrUpstreamInvalidInterval)
	_, err = svc.Create(ctx, CreateUpstreamInput{Name: "主站", BaseURL: "https://a.example", APIKey: "sk", IntervalMinutes: upstreamIntPtr(1441)})
	require.ErrorIs(t, err, ErrUpstreamInvalidInterval)

	missingProxy := int64(999)
	_, err = svc.Create(ctx, CreateUpstreamInput{Name: "主站", BaseURL: "https://a.example", APIKey: "sk", ProxyID: &missingProxy})
	require.ErrorIs(t, err, ErrUpstreamProxyNotFound)

	created, err := svc.Create(ctx, CreateUpstreamInput{Name: "主站", BaseURL: "https://a.example", APIKey: "sk"})
	require.NoError(t, err)
	require.Equal(t, 15, created.IntervalMinutes)
	require.True(t, created.Enabled)
}

func TestUpstreamServiceScheduledProbeHonorsEnabledAndDueAt(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	requests := 0
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		requests++
		return upstreamJSONResponse(200, `{"mode":"unrestricted","balance":1}`)
	}}
	svc, _ := newUpstreamTestService(repo, httpStub, nil)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a"})
	require.NoError(t, err)
	require.NoError(t, svc.RunDue(context.Background()))
	require.Equal(t, 1, requests)

	// 未到期不重复探测。
	require.NoError(t, svc.RunDue(context.Background()))
	require.Equal(t, 1, requests)

	// 关闭只停定时；手动刷新仍可用。
	disabled := false
	_, err = svc.Update(context.Background(), created.ID, UpdateUpstreamInput{Enabled: &disabled})
	require.NoError(t, err)
	repo.items[created.ID].NextProbeAt = nil
	require.NoError(t, svc.RunDue(context.Background()))
	require.Equal(t, 1, requests)
	_, err = svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, 2, requests)
}

func TestUpstreamServiceDeactivateProxyMarksFailure(t *testing.T) {
	proxyID := int64(7)
	repo := &upstreamRepoStub{items: map[int64]*Upstream{}}
	httpStub := &upstreamHTTPStub{request: func(*http.Request, string) *http.Response {
		return upstreamJSONResponse(200, `{"mode":"unrestricted","balance":1}`)
	}}
	proxyRepo := &upstreamProxyRepoStub{proxies: map[int64]*Proxy{proxyID: {
		ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 8080, Status: "inactive",
	}}}
	svc, _ := newUpstreamTestService(repo, httpStub, proxyRepo)
	created, err := svc.Create(context.Background(), CreateUpstreamInput{
		Name: "主站", BaseURL: "https://upstream.example", APIKey: "sk-a", ProxyID: &proxyID,
	})
	require.NoError(t, err)
	snapshot, err := svc.ProbeUpstream(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", snapshot.Status)
	require.Equal(t, "proxy_unavailable", snapshot.LastError)
	require.True(t, proxyRepo.getCalled)
}

func TestUpstreamServiceMaskFallbackWhenKeyUndecryptable(t *testing.T) {
	repo := &upstreamRepoStub{items: map[int64]*Upstream{1: {
		ID: 1, Name: "主站", BaseURL: "https://upstream.example", APIKeyEncrypted: "legacy-ciphertext", ConfigVersion: "v1", Enabled: true, IntervalMinutes: 15,
	}}}
	svc, _ := newUpstreamTestService(repo, &upstreamHTTPStub{}, nil)
	item, err := svc.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "***", item.APIKeyMasked)
	require.True(t, item.APIKeyConfigured())
}

func upstreamIntPtr(value int) *int { return &value }
