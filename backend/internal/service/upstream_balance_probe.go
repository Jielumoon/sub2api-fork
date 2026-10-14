package service

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// UpstreamBalanceProbeData 是单个额度项的解析结果，金额为上游原始 USD，由上游实体探测复用。
type UpstreamBalanceProbeData struct {
	Remaining      *float64 `json:"remaining,omitempty"`
	Limit          *float64 `json:"limit,omitempty"`
	Used           *float64 `json:"used,omitempty"`
	Unlimited      bool     `json:"unlimited,omitempty"`
	Unit           string   `json:"unit"`
	ResetAt        string   `json:"reset_at,omitempty"`
	ExpiresAt      string   `json:"expires_at,omitempty"`
	UpstreamStatus string   `json:"upstream_status,omitempty"`
}

// RawMessage 区分“没有此字段”和明确的 null（订阅没有周期限额）。
func parseUpstreamBalance(body []byte, kind string) (*UpstreamBalanceProbeData, string) {
	var response struct {
		Mode       string                    `json:"mode"`
		Status     string                    `json:"status"`
		Unit       string                    `json:"unit"`
		Balance    *float64                  `json:"balance"`
		Remaining  *float64                  `json:"remaining"`
		Quota      *UpstreamBalanceProbeData `json:"quota"`
		RateLimits []struct {
			Window string `json:"window"`
			UpstreamBalanceProbeData
		} `json:"rate_limits"`
		Subscription map[string]json.RawMessage `json:"subscription"`
		ExpiresAt    string                     `json:"expires_at"`
	}
	if json.Unmarshal(body, &response) != nil {
		return nil, "invalid_response"
	}
	if response.Mode != "unrestricted" && response.Mode != "quota_limited" {
		return nil, "unsupported"
	}
	if response.Unit != "" && response.Unit != "USD" {
		return nil, "invalid_response"
	}
	data := &UpstreamBalanceProbeData{Unit: "USD", ExpiresAt: validProbeTimestamp(response.ExpiresAt)}
	switch response.Status {
	case "active", "disabled", "expired", "quota_exhausted":
		data.UpstreamStatus = response.Status
	}
	switch {
	case kind == "wallet":
		if response.Mode != "unrestricted" || response.Balance == nil {
			return nil, "unavailable"
		}
		data.Remaining = response.Balance
	case strings.HasPrefix(kind, "key_"):
		if response.Mode != "quota_limited" {
			return nil, "unavailable"
		}
		selected := response.Quota
		if kind != "key_total" {
			selected = nil
			for i := range response.RateLimits {
				if response.RateLimits[i].Window == strings.TrimPrefix(kind, "key_") {
					if selected != nil {
						return nil, "invalid_response"
					}
					selected = &response.RateLimits[i].UpstreamBalanceProbeData
				}
			}
		}
		if selected == nil || selected.Remaining == nil || selected.Limit == nil || selected.Used == nil {
			return nil, "unavailable"
		}
		if selected.Unit != "" && selected.Unit != "USD" {
			return nil, "invalid_response"
		}
		data.Remaining, data.Limit, data.Used = selected.Remaining, selected.Limit, selected.Used
		data.ResetAt = validProbeTimestamp(selected.ResetAt)
	case strings.HasPrefix(kind, "subscription_"):
		if response.Mode != "unrestricted" || len(response.Subscription) == 0 {
			return nil, "unavailable"
		}
		var expiresAt string
		_ = json.Unmarshal(response.Subscription["expires_at"], &expiresAt)
		data.ExpiresAt = validProbeTimestamp(expiresAt)
		if kind == "subscription_remaining" {
			if response.Remaining == nil {
				return nil, "unavailable"
			}
			if *response.Remaining == -1 {
				data.Unlimited = true
			} else {
				data.Remaining = response.Remaining
			}
		} else {
			period := strings.TrimPrefix(kind, "subscription_")
			limit, ok := response.Subscription[period+"_limit_usd"]
			usage, hasUsage := response.Subscription[period+"_usage_usd"]
			if !ok || !hasUsage {
				return nil, "unavailable"
			}
			if json.Unmarshal(limit, &data.Limit) != nil || json.Unmarshal(usage, &data.Used) != nil || data.Used == nil {
				return nil, "invalid_response"
			}
			if data.Limit == nil || *data.Limit == 0 {
				data.Unlimited = true
			} else {
				remaining := math.Max(0, *data.Limit-*data.Used)
				data.Remaining = &remaining
			}
		}
	default:
		return nil, "unavailable"
	}
	for _, value := range []*float64{data.Remaining, data.Limit, data.Used} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || (kind != "wallet" && *value < 0)) {
			return nil, "invalid_response"
		}
	}
	return data, ""
}

func validProbeTimestamp(value string) string {
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return ""
	}
	return value
}

// 倍率与额度共享代理、SSRF 校验、TLS 画像及禁止重定向的传输边界。
func (s *UpstreamBillingProbeService) requestUpstreamProbe(ctx context.Context, account *Account, endpoint string, now time.Time) ([]byte, int, time.Duration, string, error) {
	if s.accountTestService == nil || s.accountTestService.httpUpstream == nil {
		return nil, 0, 0, "transport_unavailable", nil
	}
	key := account.GetCredential("api_key")
	if key == "" {
		return nil, 0, 0, "missing_api_key", nil
	}
	baseURL := account.GetCredential("base_url")
	if account.RoutesProtocolByInbound() && account.IsAdaptiveAPIProtocol() {
		baseURL = account.GetCNProtocolBaseURL(APIProtocolChatCompletions)
	}
	if account.Platform == PlatformOpenAI {
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
	} else if upstreamBillingProbeTargetIsOfficialAPI(baseURL) {
		return nil, 0, 0, "unsupported", nil
	}
	normalized, err := s.accountTestService.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, 0, 0, "invalid_base_url", nil
	}
	probeURL := buildOpenAIEndpointURL(normalized, endpoint)
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil {
			return nil, 0, 0, "proxy_unavailable", nil
		}
		if account.Proxy.ID != *account.ProxyID {
			return nil, 0, 0, "", ErrUpstreamBillingProbeIdentityChanged
		}
		proxyURL = account.Proxy.URL()
	}
	profile := HTTPUpstreamProfileDefault
	if account.Platform == PlatformOpenAI {
		profile = HTTPUpstreamProfileOpenAI
	}
	var tlsProfile *tlsfingerprint.Profile
	if s.accountTestService.tlsFPProfileService != nil {
		tlsProfile = s.accountTestService.tlsFPProfileService.ResolveTLSProfile(account)
	}
	body, status, retry, reason := s.accountTestService.sendUpstreamProbe(ctx, upstreamProbeRequest{
		probeURL:     probeURL,
		apiKey:       key,
		proxyURL:     proxyURL,
		accountID:    account.ID,
		concurrency:  account.Concurrency,
		profile:      profile,
		tlsProfile:   tlsProfile,
		applyHeaders: account.ApplyHeaderOverrides,
	}, now)
	return body, status, retry, reason, nil
}

// upstreamProbeRequest 是对 Sub2API 上游发起只读探测的出站参数，账号与上游实体共用。
// probeURL 必须已经过 validateUpstreamBaseURL 校验。
type upstreamProbeRequest struct {
	probeURL     string
	apiKey       string
	proxyURL     string
	accountID    int64
	concurrency  int
	profile      HTTPUpstreamProfile
	tlsProfile   *tlsfingerprint.Profile
	applyHeaders func(http.Header)
}

// sendUpstreamProbe 统一执行出站信封：超时、禁止重定向、body 上限、404/405 视为 unsupported。
// 只返回受控错误码，不把上游正文放进错误信息。
func (s *AccountTestService) sendUpstreamProbe(ctx context.Context, in upstreamProbeRequest, now time.Time) ([]byte, int, time.Duration, string) {
	if s == nil || s.httpUpstream == nil {
		return nil, 0, 0, "transport_unavailable"
	}
	requestCtx, cancel := context.WithTimeout(ctx, upstreamBillingProbeRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, in.probeURL, nil)
	if err != nil {
		return nil, 0, 0, "request_build_failed"
	}
	req.Header.Set("Authorization", "Bearer "+in.apiKey)
	req.Header.Set("Accept", "application/json")
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(req.Context(), in.profile)))
	if in.applyHeaders != nil {
		in.applyHeaders(req.Header)
	}
	resp, err := s.httpUpstream.DoWithTLS(req, in.proxyURL, in.accountID, in.concurrency, in.tlsProfile)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, 0, 0, "request_failed"
	}
	if resp == nil || resp.Body == nil {
		return nil, 0, 0, "empty_response"
	}
	defer func() { _ = resp.Body.Close() }()
	retry := retryAfter(resp.Header, now)
	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamBillingProbeMaxBodyBytes+1))
	if err != nil {
		return nil, resp.StatusCode, retry, "response_read_failed"
	}
	if len(body) > upstreamBillingProbeMaxBodyBytes {
		return nil, resp.StatusCode, retry, "response_too_large"
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return nil, resp.StatusCode, retry, "unsupported"
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, retry, "http_error"
	}
	return body, resp.StatusCode, retry, ""
}
