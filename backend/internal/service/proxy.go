package service

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	FallbackModeNone   = "none"
	FallbackModeProxy  = "proxy"
	FallbackModeDirect = "direct"
)

// ProxyPlaceholderAccountID 代理用户名里的账号占位符：代表某个账号发请求时替换成账号 ID，
// 让一条粘性代理（如 Resin 的 `Default.{account_id}`）给每个账号分配固定出口。
// 只用永不变化的 ID：身份一旦依赖可变字段（如账号名），探测/用量快照的身份校验都要跟着改。
const ProxyPlaceholderAccountID = "{account_id}"

type Proxy struct {
	ID             int64
	Name           string
	Protocol       string
	Host           string
	Port           int
	Username       string
	Password       string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      *time.Time
	FallbackMode   string
	BackupProxyID  *int64
	ExpiryWarnDays int
}

func (p *Proxy) IsActive() bool {
	return p.Status == StatusActive
}

// IsExpired 报告代理是否已过期（基于 expires_at，与 status 无关）。
func (p *Proxy) IsExpired(now time.Time) bool {
	return p.ExpiresAt != nil && !p.ExpiresAt.After(now)
}

func (p *Proxy) URL() string {
	u := &url.URL{
		Scheme: p.Protocol,
		Host:   net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
	}
	// 没有账号上下文（OAuth 授权、代理测试等）时占位符渲染为空，Resin 会走随机出口。
	username := strings.ReplaceAll(p.Username, ProxyPlaceholderAccountID, "")
	if username != "" && p.Password != "" {
		u.User = url.UserPassword(username, p.Password)
	}
	return u.String()
}

// HasAccountPlaceholder 报告用户名是否含账号占位符。
func (p *Proxy) HasAccountPlaceholder() bool {
	return strings.Contains(p.Username, ProxyPlaceholderAccountID)
}

// ForAccount 返回用户名按账号渲染后的副本；账号为空或用户名不含占位符时原样返回。
// 同一批加载的账号共享同一个 *Proxy，必须复制而不能原地修改。
func (p *Proxy) ForAccount(account *Account) *Proxy {
	if p == nil || account == nil || !p.HasAccountPlaceholder() {
		return p
	}
	out := *p
	out.Username = RenderProxyUsername(p.Username, account)
	return &out
}

// RenderProxyUsername 按账号替换用户名里的占位符；account 为 nil 时原样返回。
// 影子账号透传母账号凭据，{account_id} 用母账号 ID，保证同一份凭据只从一个出口发出。
func RenderProxyUsername(username string, account *Account) string {
	if account == nil {
		return username
	}
	accountID := account.ID
	if account.IsShadow() {
		accountID = *account.ParentAccountID
	}
	return strings.ReplaceAll(username, ProxyPlaceholderAccountID, strconv.FormatInt(accountID, 10))
}

type ProxyWithAccountCount struct {
	Proxy
	AccountCount   int64
	LatencyMs      *int64
	LatencyStatus  string
	LatencyMessage string
	IPAddress      string
	Country        string
	CountryCode    string
	Region         string
	City           string
	QualityStatus  string
	QualityScore   *int
	QualityGrade   string
	QualitySummary string
	QualityChecked *int64
}

type ProxyAccountSummary struct {
	ID       int64
	Name     string
	Platform string
	Type     string
	Notes    *string
}
