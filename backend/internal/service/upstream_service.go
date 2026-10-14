package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

const (
	upstreamNameMaxRunes          = 100
	upstreamBaseURLMaxLen         = 500
	upstreamAPIKeyMaxLen          = 2000
	upstreamDefaultIntervalMinute = 15
	upstreamMinIntervalMinutes    = 5
	upstreamMaxIntervalMinutes    = 24 * 60
	upstreamAPIKeyMaskPrefix      = 4
	upstreamAPIKeyMaskSuffix      = "***"
)

var (
	ErrUpstreamNotFound        = infraerrors.NotFound("UPSTREAM_NOT_FOUND", "upstream not found")
	ErrUpstreamInvalidName     = infraerrors.BadRequest("UPSTREAM_INVALID_NAME", fmt.Sprintf("name is required and must be at most %d characters", upstreamNameMaxRunes))
	ErrUpstreamInvalidBaseURL  = infraerrors.BadRequest("UPSTREAM_INVALID_BASE_URL", "base_url is invalid or not allowed")
	ErrUpstreamInvalidAPIKey   = infraerrors.BadRequest("UPSTREAM_INVALID_API_KEY", fmt.Sprintf("api_key is required and must be at most %d characters", upstreamAPIKeyMaxLen))
	ErrUpstreamInvalidInterval = infraerrors.BadRequest(
		"UPSTREAM_INVALID_INTERVAL",
		fmt.Sprintf("interval_minutes must be between %d and %d", upstreamMinIntervalMinutes, upstreamMaxIntervalMinutes),
	)
	ErrUpstreamProxyNotFound        = infraerrors.BadRequest("UPSTREAM_PROXY_NOT_FOUND", "proxy not found")
	ErrUpstreamProbeIdentityChanged = infraerrors.Conflict("UPSTREAM_PROBE_IDENTITY_CHANGED", "upstream configuration changed during probe; retry the probe")
	ErrUpstreamProbeBusy            = infraerrors.Conflict("UPSTREAM_PROBE_BUSY", "upstream is being probed, please refresh later")
	ErrUpstreamProbeUnavailable     = infraerrors.ServiceUnavailable("UPSTREAM_PROBE_UNAVAILABLE", "upstream probe is unavailable")
)

// Upstream 上游实体：站点地址 + 查询用 API Key，是余额／额度快照的真实归属。
type Upstream struct {
	ID      int64
	Name    string
	BaseURL string
	// APIKeyEncrypted 是 AES-256-GCM 密文，只在 service 内部解密用于探测，永不出现在响应里。
	APIKeyEncrypted string
	// APIKeyMasked 由 service 填充（前 4 字符 + ***），供 handler 直接输出。
	APIKeyMasked    string
	ProxyID         *int64
	Enabled         bool
	IntervalMinutes int
	ConfigVersion   string
	Probe           *UpstreamProbeSnapshot
	NextProbeAt     *time.Time
	LastAttemptAt   *time.Time
	FailureCount    int
	LastError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// APIKeyConfigured 报告是否已保存查询用 Key。
func (u *Upstream) APIKeyConfigured() bool {
	return u != nil && u.APIKeyEncrypted != ""
}

// UpstreamUpdate 是仓储更新入参。APIKeyEncrypted 为 nil 表示不轮换。
// 仓储在同一条 UPDATE 里比较地址／Key／代理与库内旧值，变化即换 NewConfigVersion 并清空快照。
type UpstreamUpdate struct {
	ID               int64
	Name             string
	BaseURL          string
	APIKeyEncrypted  *string
	ProxyID          *int64
	Enabled          bool
	IntervalMinutes  int
	NewConfigVersion string
}

// UpstreamRepository 上游持久化接口。
type UpstreamRepository interface {
	Create(ctx context.Context, upstream *Upstream) error
	GetByID(ctx context.Context, id int64) (*Upstream, error)
	List(ctx context.Context, params pagination.PaginationParams, search string) ([]Upstream, *pagination.PaginationResult, error)
	// Update 返回 configChanged：地址、Key 或代理是否变化（变化时已换版本并清空快照）。
	Update(ctx context.Context, in UpstreamUpdate) (bool, error)
	// Delete 软删除并清理快照与调度列。
	Delete(ctx context.Context, id int64) error
	// ListDue 返回已启用且到期（next_probe_at 为空或不晚于 now）的上游。
	ListDue(ctx context.Context, now time.Time, limit int) ([]Upstream, error)
	// UpdateProbeResult 以 config_version 和代理身份做 CAS 写回快照；不匹配返回 ErrUpstreamProbeIdentityChanged。
	// proxy 为探测开始时使用的代理记录（未渲染），proxyID 非空而 proxy 为 nil 表示当时代理不可用。
	UpdateProbeResult(ctx context.Context, id int64, configVersion string, proxyID *int64, proxy *Proxy, snapshot *UpstreamProbeSnapshot) error
}

// CreateUpstreamInput 创建上游入参。
type CreateUpstreamInput struct {
	Name            string
	BaseURL         string
	APIKey          string
	ProxyID         *int64
	Enabled         *bool
	IntervalMinutes *int
}

// UpdateUpstreamInput 更新上游入参；nil 字段表示不修改。APIKey 为 nil 或空白表示不轮换。
type UpdateUpstreamInput struct {
	Name            *string
	BaseURL         *string
	APIKey          *string
	SetProxy        bool
	ProxyID         *int64
	Enabled         *bool
	IntervalMinutes *int
}

// UpstreamService 上游 CRUD、探测与定时调度。
type UpstreamService struct {
	repo               UpstreamRepository
	proxyRepo          ProxyRepository
	accountTestService *AccountTestService
	encryptor          SecretEncryptor

	parentCtx    context.Context
	parentCancel context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
	started      bool
	stopped      bool
	cycleMu      sync.Mutex
	probeGroup   singleflight.Group
	probeSlots   chan struct{}
	// probeAsync 是创建／改配置后的异步探测入口，测试里替换为 no-op 以免真实发请求。
	probeAsync func(int64)
	now        func() time.Time
	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewUpstreamService(repo UpstreamRepository, proxyRepo ProxyRepository, accountTestService *AccountTestService, encryptor SecretEncryptor) *UpstreamService {
	ctx, cancel := context.WithCancel(context.Background())
	svc := &UpstreamService{
		repo:               repo,
		proxyRepo:          proxyRepo,
		accountTestService: accountTestService,
		encryptor:          encryptor,
		parentCtx:          ctx,
		parentCancel:       cancel,
		probeSlots:         make(chan struct{}, upstreamBillingProbeConcurrency),
		now:                time.Now,
		instanceID:         uuid.NewString(),
	}
	svc.probeAsync = svc.runBackgroundProbe
	return svc
}

// SetLeaderLock 注入跨实例领导锁；两者都为 nil 时单实例无门禁运行。
func (s *UpstreamService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// MaskUpstreamAPIKey 返回前 4 字符 + ***；不足 4 字符时只返回 ***。
func MaskUpstreamAPIKey(plain string) string {
	if len(plain) <= upstreamAPIKeyMaskPrefix {
		return upstreamAPIKeyMaskSuffix
	}
	return plain[:upstreamAPIKeyMaskPrefix] + upstreamAPIKeyMaskSuffix
}

// ---------- CRUD ----------

func (s *UpstreamService) List(ctx context.Context, params pagination.PaginationParams, search string) ([]Upstream, *pagination.PaginationResult, error) {
	items, result, err := s.repo.List(ctx, params, strings.TrimSpace(search))
	if err != nil {
		return nil, nil, err
	}
	for i := range items {
		s.fillMask(&items[i])
	}
	return items, result, nil
}

func (s *UpstreamService) Get(ctx context.Context, id int64) (*Upstream, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.fillMask(u)
	return u, nil
}

func (s *UpstreamService) Create(ctx context.Context, in CreateUpstreamInput) (*Upstream, error) {
	name, err := normalizeUpstreamName(in.Name)
	if err != nil {
		return nil, err
	}
	baseURL, err := s.normalizeBaseURL(in.BaseURL)
	if err != nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(in.APIKey)
	if apiKey == "" || len(apiKey) > upstreamAPIKeyMaxLen {
		return nil, ErrUpstreamInvalidAPIKey
	}
	interval := upstreamDefaultIntervalMinute
	if in.IntervalMinutes != nil {
		interval = *in.IntervalMinutes
	}
	if err := validateUpstreamInterval(interval); err != nil {
		return nil, err
	}
	if err := s.validateProxy(ctx, in.ProxyID); err != nil {
		return nil, err
	}
	encrypted, err := s.encryptor.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt upstream api key: %w", err)
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	u := &Upstream{
		Name:            name,
		BaseURL:         baseURL,
		APIKeyEncrypted: encrypted,
		ProxyID:         in.ProxyID,
		Enabled:         enabled,
		IntervalMinutes: interval,
		ConfigVersion:   uuid.NewString(),
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	u.APIKeyMasked = MaskUpstreamAPIKey(apiKey)
	s.triggerProbe(u.ID)
	return u, nil
}

func (s *UpstreamService) Update(ctx context.Context, id int64, in UpdateUpstreamInput) (*Upstream, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	update := UpstreamUpdate{
		ID:               id,
		Name:             existing.Name,
		BaseURL:          existing.BaseURL,
		ProxyID:          existing.ProxyID,
		Enabled:          existing.Enabled,
		IntervalMinutes:  existing.IntervalMinutes,
		NewConfigVersion: uuid.NewString(),
	}
	if in.Name != nil {
		if update.Name, err = normalizeUpstreamName(*in.Name); err != nil {
			return nil, err
		}
	}
	if in.BaseURL != nil {
		if update.BaseURL, err = s.normalizeBaseURL(*in.BaseURL); err != nil {
			return nil, err
		}
	}
	if in.IntervalMinutes != nil {
		if err := validateUpstreamInterval(*in.IntervalMinutes); err != nil {
			return nil, err
		}
		update.IntervalMinutes = *in.IntervalMinutes
	}
	if in.Enabled != nil {
		update.Enabled = *in.Enabled
	}
	if in.SetProxy {
		if err := s.validateProxy(ctx, in.ProxyID); err != nil {
			return nil, err
		}
		update.ProxyID = in.ProxyID
	}
	// 空 Key 表示不轮换（同 ChannelMonitor 语义）；非空则加密覆盖并视为配置变更。
	if in.APIKey != nil {
		if plain := strings.TrimSpace(*in.APIKey); plain != "" {
			if len(plain) > upstreamAPIKeyMaxLen {
				return nil, ErrUpstreamInvalidAPIKey
			}
			encrypted, encErr := s.encryptor.Encrypt(plain)
			if encErr != nil {
				return nil, fmt.Errorf("encrypt upstream api key: %w", encErr)
			}
			update.APIKeyEncrypted = &encrypted
		}
	}
	changed, err := s.repo.Update(ctx, update)
	if err != nil {
		return nil, err
	}
	if changed {
		s.triggerProbe(id)
	}
	return s.Get(ctx, id)
}

func (s *UpstreamService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *UpstreamService) fillMask(u *Upstream) {
	if u == nil || u.APIKeyEncrypted == "" {
		return
	}
	plain, err := s.encryptor.Decrypt(u.APIKeyEncrypted)
	if err != nil {
		// 解密失败（如密钥被更换）只给占位掩码，仍报告已配置，提示管理员重新填写。
		u.APIKeyMasked = upstreamAPIKeyMaskSuffix
		return
	}
	u.APIKeyMasked = MaskUpstreamAPIKey(plain)
}

func normalizeUpstreamName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > upstreamNameMaxRunes {
		return "", ErrUpstreamInvalidName
	}
	return name, nil
}

func validateUpstreamInterval(minutes int) error {
	if minutes < upstreamMinIntervalMinutes || minutes > upstreamMaxIntervalMinutes {
		return ErrUpstreamInvalidInterval
	}
	return nil
}

// normalizeBaseURL 与账号探测同一套 SSRF／白名单校验，保存规范化后的地址。
func (s *UpstreamService) normalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > upstreamBaseURLMaxLen || s.accountTestService == nil {
		return "", ErrUpstreamInvalidBaseURL
	}
	normalized, err := s.accountTestService.validateUpstreamBaseURL(trimmed)
	if err != nil {
		return "", ErrUpstreamInvalidBaseURL
	}
	return normalized, nil
}

func (s *UpstreamService) validateProxy(ctx context.Context, proxyID *int64) error {
	if proxyID == nil {
		return nil
	}
	if s.proxyRepo == nil {
		return ErrUpstreamProxyNotFound
	}
	if _, err := s.proxyRepo.GetByID(ctx, *proxyID); err != nil {
		if errors.Is(err, ErrProxyNotFound) {
			return ErrUpstreamProxyNotFound
		}
		return err
	}
	return nil
}
