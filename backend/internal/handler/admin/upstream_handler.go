package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const upstreamMaxPageSize = 100

// UpstreamHandler 上游实体管理后台 handler。
type UpstreamHandler struct {
	upstreamService *service.UpstreamService
}

func NewUpstreamHandler(upstreamService *service.UpstreamService) *UpstreamHandler {
	return &UpstreamHandler{upstreamService: upstreamService}
}

// --- Request / Response ---

type upstreamCreateRequest struct {
	Name            string `json:"name" binding:"required,max=100"`
	BaseURL         string `json:"base_url" binding:"required,max=500"`
	APIKey          string `json:"api_key" binding:"required,max=2000"`
	ProxyID         *int64 `json:"proxy_id"`
	Enabled         *bool  `json:"enabled"`
	IntervalMinutes *int   `json:"interval_minutes" binding:"omitempty,min=5,max=1440"`
}

// upstreamUpdateRequest 中 api_key 缺省或空串表示不轮换。
type upstreamUpdateRequest struct {
	Name            *string                `json:"name" binding:"omitempty,max=100"`
	BaseURL         *string                `json:"base_url" binding:"omitempty,max=500"`
	APIKey          *string                `json:"api_key" binding:"omitempty,max=2000"`
	ProxyID         dto.NullableInt64Field `json:"proxy_id"`
	Enabled         *bool                  `json:"enabled"`
	IntervalMinutes *int                   `json:"interval_minutes" binding:"omitempty,min=5,max=1440"`
}

// upstreamResponse 只暴露掩码 Key，绝不返回 api_key_encrypted 或明文。
type upstreamResponse struct {
	ID               int64                          `json:"id"`
	Name             string                         `json:"name"`
	BaseURL          string                         `json:"base_url"`
	APIKeyMasked     string                         `json:"api_key_masked"`
	APIKeyConfigured bool                           `json:"api_key_configured"`
	ProxyID          *int64                         `json:"proxy_id"`
	Enabled          bool                           `json:"enabled"`
	IntervalMinutes  int                            `json:"interval_minutes"`
	Probe            *service.UpstreamProbeSnapshot `json:"probe,omitempty"`
	NextProbeAt      *string                        `json:"next_probe_at"`
	LastAttemptAt    *string                        `json:"last_attempt_at"`
	FailureCount     int                            `json:"failure_count"`
	LastError        string                         `json:"last_error"`
	CreatedAt        string                         `json:"created_at"`
	UpdatedAt        string                         `json:"updated_at"`
}

func upstreamToResponse(u *service.Upstream) *upstreamResponse {
	if u == nil {
		return nil
	}
	resp := &upstreamResponse{
		ID:               u.ID,
		Name:             u.Name,
		BaseURL:          u.BaseURL,
		APIKeyMasked:     u.APIKeyMasked,
		APIKeyConfigured: u.APIKeyConfigured(),
		ProxyID:          u.ProxyID,
		Enabled:          u.Enabled,
		IntervalMinutes:  u.IntervalMinutes,
		Probe:            u.Probe,
		FailureCount:     u.FailureCount,
		LastError:        u.LastError,
		CreatedAt:        u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        u.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if u.NextProbeAt != nil {
		value := u.NextProbeAt.UTC().Format(time.RFC3339)
		resp.NextProbeAt = &value
	}
	if u.LastAttemptAt != nil {
		value := u.LastAttemptAt.UTC().Format(time.RFC3339)
		resp.LastAttemptAt = &value
	}
	return resp
}

// parseUpstreamID 提取并校验路径参数 :id；校验失败时已写入 4xx 响应。
func parseUpstreamID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_UPSTREAM_ID", "invalid upstream id"))
		return 0, false
	}
	return id, true
}

// --- Handlers ---

// List GET /api/v1/admin/upstreams
func (h *UpstreamHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > upstreamMaxPageSize {
		pageSize = upstreamMaxPageSize
	}
	search := strings.TrimSpace(c.Query("search"))
	// 按字符截断（同 user_handler.go），避免把中文截成非法 UTF-8 导致 PG 报错。
	if runes := []rune(search); len(runes) > 100 {
		search = string(runes[:100])
	}
	items, result, err := h.upstreamService.List(c.Request.Context(), pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "id"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]upstreamResponse, 0, len(items))
	for i := range items {
		out = append(out, *upstreamToResponse(&items[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// GetByID GET /api/v1/admin/upstreams/:id
func (h *UpstreamHandler) GetByID(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	u, err := h.upstreamService.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, upstreamToResponse(u))
}

// Create POST /api/v1/admin/upstreams
func (h *UpstreamHandler) Create(c *gin.Context) {
	var req upstreamCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_UPSTREAM_REQUEST", err.Error()))
		return
	}
	u, err := h.upstreamService.Create(c.Request.Context(), service.CreateUpstreamInput{
		Name:            req.Name,
		BaseURL:         req.BaseURL,
		APIKey:          req.APIKey,
		ProxyID:         req.ProxyID,
		Enabled:         req.Enabled,
		IntervalMinutes: req.IntervalMinutes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, upstreamToResponse(u))
}

// Update PUT /api/v1/admin/upstreams/:id
func (h *UpstreamHandler) Update(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	var req upstreamUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_UPSTREAM_REQUEST", err.Error()))
		return
	}
	u, err := h.upstreamService.Update(c.Request.Context(), id, service.UpdateUpstreamInput{
		Name:            req.Name,
		BaseURL:         req.BaseURL,
		APIKey:          req.APIKey,
		SetProxy:        req.ProxyID.Set,
		ProxyID:         req.ProxyID.Value,
		Enabled:         req.Enabled,
		IntervalMinutes: req.IntervalMinutes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, upstreamToResponse(u))
}

// Delete DELETE /api/v1/admin/upstreams/:id
func (h *UpstreamHandler) Delete(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	if err := h.upstreamService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

// Probe POST /api/v1/admin/upstreams/:id/probe —— 手动探测，忽略 enabled 与到期时间。
func (h *UpstreamHandler) Probe(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	snapshot, err := h.upstreamService.ProbeUpstream(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"upstream_id": id, "snapshot": snapshot})
}
