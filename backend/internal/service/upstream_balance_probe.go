package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	UpstreamBalanceProbeTypeExtraKey     = "upstream_balance_probe_type"
	UpstreamBalanceProbeExtraKey         = "upstream_balance_probe"
	UpstreamBalanceProbeRevisionExtraKey = "upstream_balance_probe_revision"
)

var ErrUpstreamBalanceProbeDisabled = infraerrors.BadRequest("UPSTREAM_BALANCE_PROBE_DISABLED", "select an upstream balance type first")

// ValidateUpstreamBalanceProbeType 在所有账号配置写入入口使用同一白名单。
func ValidateUpstreamBalanceProbeType(value any) error {
	kind, ok := value.(string)
	if !ok {
		return infraerrors.BadRequest("INVALID_UPSTREAM_BALANCE_PROBE_TYPE", "upstream balance type must be a string")
	}
	switch kind {
	case "disabled", "wallet", "key_total", "key_5h", "key_1d", "key_7d",
		"subscription_daily", "subscription_weekly", "subscription_monthly", "subscription_remaining":
		return nil
	default:
		return infraerrors.BadRequest("INVALID_UPSTREAM_BALANCE_PROBE_TYPE", "invalid upstream balance type")
	}
}

func UpstreamBalanceProbeType(account *Account) string {
	if !isUpstreamBillingProbeAccount(account) {
		return "disabled"
	}
	value, _ := account.Extra[UpstreamBalanceProbeTypeExtraKey].(string)
	if ValidateUpstreamBalanceProbeType(value) != nil {
		return "disabled"
	}
	return value
}

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

type UpstreamBalanceProbeSnapshot struct {
	Type          string                    `json:"type"`
	Status        string                    `json:"status"`
	Data          *UpstreamBalanceProbeData `json:"data,omitempty"`
	ReceivedAt    *time.Time                `json:"received_at,omitempty"`
	FreshUntil    *time.Time                `json:"fresh_until,omitempty"`
	LastAttemptAt time.Time                 `json:"last_attempt_at"`
	NextProbeAt   time.Time                 `json:"next_probe_at"`
	FailureCount  int                       `json:"failure_count,omitempty"`
	HTTPStatus    int                       `json:"http_status,omitempty"`
	LastError     string                    `json:"last_error,omitempty"`
}

func DecodeUpstreamBalanceProbeSnapshot(account *Account) *UpstreamBalanceProbeSnapshot {
	kind := UpstreamBalanceProbeType(account)
	if kind == "disabled" {
		return nil
	}
	raw, err := json.Marshal(account.Extra[UpstreamBalanceProbeExtraKey])
	if err != nil {
		return nil
	}
	var snapshot *UpstreamBalanceProbeSnapshot
	if json.Unmarshal(raw, &snapshot) != nil || snapshot == nil || snapshot.Type != kind {
		return nil
	}
	return snapshot
}

func nextUpstreamAccountProbe(account *Account) (time.Time, bool) {
	var next time.Time
	enabled := upstreamBillingProbeEnabled(account)
	if enabled {
		if snapshot := decodeUpstreamBillingProbeSnapshot(account.Extra); snapshot != nil {
			next = snapshot.NextProbeAt
		}
	}
	if UpstreamBalanceProbeType(account) != "disabled" {
		var balanceNext time.Time
		if snapshot := DecodeUpstreamBalanceProbeSnapshot(account); snapshot != nil {
			balanceNext = snapshot.NextProbeAt
		}
		if !enabled || balanceNext.Before(next) {
			next = balanceNext
		}
		enabled = true
	}
	return next, enabled
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

type upstreamBalanceProbeRepository interface {
	UpdateUpstreamBalanceProbeSnapshot(context.Context, *Account, *UpstreamBalanceProbeSnapshot) error
}

func (s *UpstreamBillingProbeService) ProbeBalanceAccount(ctx context.Context, accountID int64) (*UpstreamBalanceProbeSnapshot, error) {
	if s == nil || s.accountRepo == nil {
		return nil, ErrUpstreamBillingProbeUnavailable
	}
	settings, err := s.getSettings(ctx)
	if err != nil {
		return nil, err
	}
	return s.probeBalanceAccount(ctx, accountID, settings.IntervalMinutes, false)
}

func (s *UpstreamBillingProbeService) probeBalanceAccount(ctx context.Context, accountID int64, interval int, scheduled bool) (*UpstreamBalanceProbeSnapshot, error) {
	value, err, _ := s.probeGroup.Do("balance:"+strconv.FormatInt(accountID, 10), func() (any, error) {
		select {
		case s.probeSlots <- struct{}{}:
			defer func() { <-s.probeSlots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		release, acquired, lockErr := s.tryAcquireLeaderLock(ctx, fmt.Sprintf("upstream:probe:account:%d", accountID))
		if lockErr != nil {
			return nil, lockErr
		}
		if !acquired {
			return nil, infraerrors.Conflict("UPSTREAM_PROBE_BUSY", "账号正在探测，请稍后刷新")
		}
		defer release()
		account, err := s.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if UpstreamBalanceProbeType(account) == "disabled" {
			if scheduled {
				return nil, nil
			}
			return nil, ErrUpstreamBalanceProbeDisabled
		}
		previous := DecodeUpstreamBalanceProbeSnapshot(account)
		if scheduled && (!account.IsActive() || (previous != nil && s.currentTime().Before(previous.NextProbeAt))) {
			return nil, nil
		}
		return s.probeLoadedBalance(ctx, account, interval)
	})
	if err != nil || value == nil {
		return nil, err
	}
	snapshot, ok := value.(*UpstreamBalanceProbeSnapshot)
	if !ok {
		return nil, fmt.Errorf("invalid upstream balance probe result")
	}
	return snapshot, nil
}

func (s *UpstreamBillingProbeService) probeLoadedBalance(ctx context.Context, account *Account, interval int) (*UpstreamBalanceProbeSnapshot, error) {
	repo, ok := s.accountRepo.(upstreamBalanceProbeRepository)
	if !ok {
		return nil, ErrUpstreamBillingProbeUnavailable
	}
	now := s.currentTime().UTC()
	snapshot := &UpstreamBalanceProbeSnapshot{Type: UpstreamBalanceProbeType(account), Status: "ok", LastAttemptAt: now}
	body, status, retry, reason, err := s.requestUpstreamProbe(ctx, account, "/v1/usage", now)
	if err != nil {
		return nil, err
	}
	snapshot.HTTPStatus = status
	if reason == "" {
		snapshot.Data, reason = parseUpstreamBalance(body, snapshot.Type)
	}
	delay := nextProbeDelay(interval, retry)
	if reason == "" {
		snapshot.ReceivedAt = probeTimePtr(now)
		snapshot.FreshUntil = probeTimePtr(now.Add(2 * time.Duration(interval) * time.Minute))
	} else {
		snapshot.Status, snapshot.LastError, snapshot.FailureCount = "failed", reason, 1
		if reason == "unsupported" || reason == "unavailable" {
			snapshot.Status = reason
			delay = unsupportedProbeDelay(interval, retry)
		}
		if previous := DecodeUpstreamBalanceProbeSnapshot(account); previous != nil {
			snapshot.FailureCount += previous.FailureCount
			if snapshot.Status == "failed" {
				snapshot.Data, snapshot.ReceivedAt, snapshot.FreshUntil = previous.Data, previous.ReceivedAt, previous.FreshUntil
			}
		}
		if snapshot.Status == "failed" {
			delay = nextProbeDelay(interval*(1<<min(snapshot.FailureCount-1, 6)), retry)
		}
	}
	snapshot.NextProbeAt = now.Add(delay)
	if err := repo.UpdateUpstreamBalanceProbeSnapshot(ctx, account, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
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

// 配置校验与普通 extra 更新共用，观测结果只能由探测器写入。
func validateUpstreamBalanceExtra(extra map[string]any, platform, accountType string) error {
	value, exists := extra[UpstreamBalanceProbeTypeExtraKey]
	if !exists {
		return nil
	}
	if err := ValidateUpstreamBalanceProbeType(value); err != nil {
		return err
	}
	if value != "disabled" && !IsUpstreamBillingProbeIdentity(platform, accountType) {
		return ErrUpstreamBillingProbeAccountInvalid
	}
	return nil
}
