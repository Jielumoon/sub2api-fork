package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"golang.org/x/sync/errgroup"
)

const (
	upstreamProbeEndpoint      = "/v1/usage"
	upstreamProbeLeaderLockKey = "upstream:entity:probe:leader"
	upstreamProbeLeaderLockTTL = 5 * time.Minute
	// 单行探测锁只需覆盖一次请求（出站超时 10s）；TTL 过长会让崩溃实例把手动刷新
	// 卡到锁自然过期。
	upstreamProbeItemLockTTL    = time.Minute
	upstreamProbeMaxPerCycle    = upstreamBillingProbeMaxPerCycle
	upstreamProbeStatusOK       = "ok"
	upstreamProbeStatusFailed   = "failed"
	upstreamProbeStatusNoSchema = "unsupported"
	upstreamProbeStatusMissing  = "unavailable"
)

// upstreamUsageItemKinds 是 /v1/usage 可解析的全部额度项，顺序即展示与 expires_at 取值顺序。
var upstreamUsageItemKinds = []string{
	"wallet", "key_total", "key_5h", "key_1d", "key_7d",
	"subscription_daily", "subscription_weekly", "subscription_monthly", "subscription_remaining",
}

// UpstreamUsageItem 是单个额度项；字段缺失（nil）表示上游未提供，不等于 0。
type UpstreamUsageItem struct {
	Remaining *float64 `json:"remaining,omitempty"`
	Limit     *float64 `json:"limit,omitempty"`
	Used      *float64 `json:"used,omitempty"`
	Unlimited bool     `json:"unlimited,omitempty"`
	ResetAt   string   `json:"reset_at,omitempty"`
}

// UpstreamUsageData 是一次 /v1/usage 响应中解析出的全部额度项，金额为上游原始 USD。
type UpstreamUsageData struct {
	Unit           string                        `json:"unit"`
	UpstreamStatus string                        `json:"upstream_status,omitempty"`
	ExpiresAt      string                        `json:"expires_at,omitempty"`
	Items          map[string]*UpstreamUsageItem `json:"items"`
}

// UpstreamProbeSnapshot 存于 upstreams.probe；调度列同时写入独立列以便走索引。
type UpstreamProbeSnapshot struct {
	Status        string             `json:"status"`
	Data          *UpstreamUsageData `json:"data,omitempty"`
	ReceivedAt    *time.Time         `json:"received_at,omitempty"`
	FreshUntil    *time.Time         `json:"fresh_until,omitempty"`
	LastAttemptAt time.Time          `json:"last_attempt_at"`
	NextProbeAt   time.Time          `json:"next_probe_at"`
	FailureCount  int                `json:"failure_count,omitempty"`
	HTTPStatus    int                `json:"http_status,omitempty"`
	LastError     string             `json:"last_error,omitempty"`
}

// parseUpstreamUsageAll 收集响应里的全部额度项，逐项复用 parseUpstreamBalance 的模式、单位与数值校验。
// 单项 unavailable 只表示该项缺失；任一项数值非法或协议未知即整体拒绝；一项都没有时为 unavailable。
func parseUpstreamUsageAll(body []byte) (*UpstreamUsageData, string) {
	var out *UpstreamUsageData
	for _, kind := range upstreamUsageItemKinds {
		item, reason := parseUpstreamBalance(body, kind)
		if reason == upstreamProbeStatusMissing {
			continue
		}
		if reason != "" {
			return nil, reason
		}
		if out == nil {
			out = &UpstreamUsageData{Unit: item.Unit, UpstreamStatus: item.UpstreamStatus, Items: map[string]*UpstreamUsageItem{}}
		}
		if out.ExpiresAt == "" {
			out.ExpiresAt = item.ExpiresAt
		}
		out.Items[kind] = &UpstreamUsageItem{
			Remaining: item.Remaining,
			Limit:     item.Limit,
			Used:      item.Used,
			Unlimited: item.Unlimited,
			ResetAt:   item.ResetAt,
		}
	}
	if out == nil {
		return nil, upstreamProbeStatusMissing
	}
	return out, ""
}

// ProvideUpstreamService 创建上游服务并启动定时探测。
func ProvideUpstreamService(
	repo UpstreamRepository,
	proxyRepo ProxyRepository,
	accountTestService *AccountTestService,
	encryptor SecretEncryptor,
	lockCache LeaderLockCache,
	db *sql.DB,
) *UpstreamService {
	svc := NewUpstreamService(repo, proxyRepo, accountTestService, encryptor)
	svc.SetLeaderLock(lockCache, db)
	svc.Start()
	return svc
}

func (s *UpstreamService) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.wg.Add(1)
	s.mu.Unlock()
	go s.runLoop()
}

func (s *UpstreamService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.parentCancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *UpstreamService) runLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(upstreamBillingProbeCycleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.parentCtx.Done():
			return
		case <-ticker.C:
			if err := s.RunDue(s.parentCtx); err != nil {
				logger.LegacyPrintf("service.upstream_probe", "run_due_failed: err=%v", err)
			}
		}
	}
}

// RunDue 在领导锁内执行一批到期探测；锁被其他实例持有时本轮直接返回。
func (s *UpstreamService) RunDue(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return nil
	}
	s.cycleMu.Lock()
	defer s.cycleMu.Unlock()
	release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, upstreamProbeLeaderLockKey, s.instanceID, upstreamProbeLeaderLockTTL)
	if !acquired {
		return nil
	}
	defer release()

	due, err := s.repo.ListDue(ctx, s.currentTime(), upstreamProbeMaxPerCycle)
	if err != nil {
		return fmt.Errorf("list due upstreams: %w", err)
	}
	var group errgroup.Group
	for i := range due {
		id := due[i].ID
		group.Go(func() error {
			if _, probeErr := s.probe(ctx, id, true); probeErr != nil {
				logger.LegacyPrintf("service.upstream_probe", "probe_due_failed: upstream_id=%d err=%v", id, probeErr)
			}
			return nil
		})
	}
	return group.Wait()
}

// ProbeUpstream 手动探测：忽略 enabled 与到期时间，返回最新快照。
func (s *UpstreamService) ProbeUpstream(ctx context.Context, id int64) (*UpstreamProbeSnapshot, error) {
	return s.probe(ctx, id, false)
}

// triggerProbe 在创建／改配置后异步探测一次；测试可替换 probeAsync 跳过。
func (s *UpstreamService) triggerProbe(id int64) {
	if s == nil || s.probeAsync == nil {
		return
	}
	s.probeAsync(id)
}

// runBackgroundProbe 使用服务自身的上下文而不是请求上下文，请求结束后仍能写回首次结果。
func (s *UpstreamService) runBackgroundProbe(id int64) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		if _, err := s.probe(s.parentCtx, id, false); err != nil && !errors.Is(err, ErrUpstreamProbeIdentityChanged) {
			logger.LegacyPrintf("service.upstream_probe", "initial_probe_failed: upstream_id=%d err=%v", id, err)
		}
	}()
}

func (s *UpstreamService) probe(ctx context.Context, id int64, scheduled bool) (*UpstreamProbeSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, ErrUpstreamProbeUnavailable
	}
	// 手动与定时分开合并：定时「未到期」返回 nil，不能被同时到达的手动请求共享成空快照。
	key := strconv.FormatInt(id, 10)
	if scheduled {
		key = "scheduled:" + key
	}
	value, err, _ := s.probeGroup.Do(key, func() (any, error) {
		select {
		case s.probeSlots <- struct{}{}:
			defer func() { <-s.probeSlots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		// 单个上游的跨实例锁：手动刷新与另一实例的定时探测不重复发请求。
		release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, fmt.Sprintf("upstream:entity:probe:%d", id), s.instanceID, upstreamProbeItemLockTTL)
		if !acquired {
			if scheduled {
				return nil, nil
			}
			return nil, ErrUpstreamProbeBusy
		}
		defer release()
		u, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if scheduled && (!u.Enabled || (u.NextProbeAt != nil && s.currentTime().Before(*u.NextProbeAt))) {
			return nil, nil
		}
		snapshot, err := s.probeLoaded(ctx, u)
		if scheduled && errors.Is(err, ErrUpstreamProbeIdentityChanged) {
			// 配置已变或已删除：丢弃迟到结果，新配置已被置为立即到期。
			return nil, nil
		}
		return snapshot, err
	})
	if err != nil || value == nil {
		return nil, err
	}
	snapshot, ok := value.(*UpstreamProbeSnapshot)
	if !ok {
		return nil, fmt.Errorf("invalid upstream probe result")
	}
	return snapshot, nil
}

func (s *UpstreamService) probeLoaded(ctx context.Context, u *Upstream) (*UpstreamProbeSnapshot, error) {
	now := s.currentTime().UTC()
	interval := u.IntervalMinutes
	proxy, body, status, retry, reason, err := s.requestProbe(ctx, u, now)
	if err != nil {
		return nil, err
	}
	snapshot := &UpstreamProbeSnapshot{Status: upstreamProbeStatusOK, LastAttemptAt: now, HTTPStatus: status}
	if reason == "" {
		snapshot.Data, reason = parseUpstreamUsageAll(body)
	}
	delay := nextProbeDelay(interval, retry)
	if reason == "" {
		snapshot.ReceivedAt = probeTimePtr(now)
		snapshot.FreshUntil = probeTimePtr(now.Add(2 * time.Duration(interval) * time.Minute))
	} else {
		snapshot.Status, snapshot.LastError, snapshot.FailureCount = upstreamProbeStatusFailed, reason, u.FailureCount+1
		if reason == upstreamProbeStatusNoSchema || reason == upstreamProbeStatusMissing {
			snapshot.Status = reason
			delay = unsupportedProbeDelay(interval, retry)
		} else {
			delay = nextProbeDelay(interval*(1<<min(snapshot.FailureCount-1, 6)), retry)
		}
		// failed（传输／解析失败）保留上一次成功值，fresh_until 过期后前端标记为陈旧；
		// unsupported / unavailable 清除旧值，避免把与当前响应不符的金额当成现状。
		if snapshot.Status == upstreamProbeStatusFailed {
			if previous := u.Probe; previous != nil {
				snapshot.Data, snapshot.ReceivedAt, snapshot.FreshUntil = previous.Data, previous.ReceivedAt, previous.FreshUntil
			}
		}
	}
	snapshot.NextProbeAt = now.Add(delay)
	if err := s.repo.UpdateProbeResult(ctx, u.ID, u.ConfigVersion, u.ProxyID, proxy, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// requestProbe 解密 Key、校验地址、解析代理后经共享出站信封请求 /v1/usage。
// 返回探测时使用的代理记录（未渲染）供写回时核对代理身份。
//
// 代理用户名模板：上游实体不属于任何账号，按无账号上下文处理，直接用 proxy.URL()，
// {account_id} 渲染为空（Resin 走随机出口）。不借用上游 ID 渲染，避免和账号 ID 撞号共用出口。
func (s *UpstreamService) requestProbe(ctx context.Context, u *Upstream, now time.Time) (*Proxy, []byte, int, time.Duration, string, error) {
	if s.accountTestService == nil || s.encryptor == nil {
		return nil, nil, 0, 0, "transport_unavailable", nil
	}
	// 先加载代理：之后任何提前返回都要带上真实代理记录，否则写回时的代理身份核对
	// 会把「代理存在、传入 nil」判成配置已变，结果永远写不进去、next_probe_at 也不前移。
	var proxy *Proxy
	proxyURL := ""
	if u.ProxyID != nil {
		if s.proxyRepo == nil {
			return nil, nil, 0, 0, "proxy_unavailable", nil
		}
		var err error
		proxy, err = s.proxyRepo.GetByID(ctx, *u.ProxyID)
		if err != nil {
			if errors.Is(err, ErrProxyNotFound) {
				return nil, nil, 0, 0, "proxy_unavailable", nil
			}
			return nil, nil, 0, 0, "", err
		}
		if !proxy.IsActive() {
			return proxy, nil, 0, 0, "proxy_unavailable", nil
		}
		proxyURL = proxy.URL()
	}
	key, err := s.encryptor.Decrypt(u.APIKeyEncrypted)
	if err != nil || key == "" {
		return proxy, nil, 0, 0, "api_key_decrypt_failed", nil
	}
	if upstreamBillingProbeTargetIsOfficialAPI(u.BaseURL) {
		return proxy, nil, 0, 0, upstreamProbeStatusNoSchema, nil
	}
	normalized, err := s.accountTestService.validateUpstreamBaseURL(u.BaseURL)
	if err != nil {
		return proxy, nil, 0, 0, "invalid_base_url", nil
	}
	body, status, retry, reason := s.accountTestService.sendUpstreamProbe(ctx, upstreamProbeRequest{
		probeURL: buildOpenAIEndpointURL(normalized, upstreamProbeEndpoint),
		apiKey:   key,
		proxyURL: proxyURL,
		profile:  HTTPUpstreamProfileDefault,
	}, now)
	return proxy, body, status, retry, reason, nil
}

func (s *UpstreamService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}
