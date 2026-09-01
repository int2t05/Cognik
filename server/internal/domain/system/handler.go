// Package system 聚合系统管理领域的 HTTP 请求处理层。
//
// handler.go 合并原 audit / config / dashboard / message 四个 Handler。
// Handler 层职责：参数解析、调用 Service、格式化响应；统计/聚合逻辑在 Service 层完成。
//
// TODO(Slice 8): parsePagination / getCurrentUserID / handleServiceError 三个工具函数
// 当前从 handler/common.go、handler/auth.go 内联复制到此，待后续抽取到共享 HTTP
// helper 包后统一消除重复。
package system

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"opsmind/internal/shared/dto/request"
	dto "opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// 内联工具函数（待 Slice 8 抽取到共享包）
// =============================================================================

// parsePagination 从查询参数中解析分页参数（page, pageSize）。
// 默认值：page=1, pageSize=10。上限：pageSize≤100。
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	return page, pageSize
}

// getCurrentUserID 从 Gin context 中获取当前用户 ID。
// JWTAuth 中间件将当前用户 ID 以 int64 类型写入 context，key 为 "userID"。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// handleServiceError 统一处理 Service 层错误。
// AppError 类型提取业务码，其他错误视为 500。
func handleServiceError(c *gin.Context, err error) {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		response.Error(c, appErr.Code, appErr.Message)
		return
	}
	// 非 AppError 说明是未预期的内部错误，记录真实原因方便排查
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	response.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// =============================================================================
// 审计日志 AuditHandler
// =============================================================================

// AuditHandler 审计日志查询接口。
type AuditHandler struct {
	svc *AuditService
}

// NewAuditHandler 创建 AuditHandler 实例。
func NewAuditHandler(svc *AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List 查询审计日志列表（支持多维过滤和日期范围）。
//
// GET /api/v1/admin/audit-logs?operator_id=1&action=user.create&target_type=user&target_id=42&date_from=2026-01-01&date_to=2026-06-30
func (h *AuditHandler) List(c *gin.Context) {
	var req request.AuditLogListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	page, pageSize := parsePagination(c)

	f := AuditFilter{
		OperatorID: req.OperatorID,
		Action:     req.Action,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		Page:       page,
		PageSize:   pageSize,
	}

	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.SuccessWithPage(c, items, total, page, pageSize)
}

// BatchDelete 批量删除审计日志。
//
// POST /api/v1/admin/audit-logs/batch-delete
func (h *AuditHandler) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}
	deleted, err := h.svc.BatchDelete(c.Request.Context(), req.IDs)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, map[string]int64{"deleted": deleted})
}

// =============================================================================
// 系统配置 ConfigHandler
// =============================================================================

// ConfigHandler 系统配置管理接口。
type ConfigHandler struct {
	svc *ConfigService
}

// NewConfigHandler 创建 ConfigHandler 实例。
func NewConfigHandler(svc *ConfigService) *ConfigHandler {
	return &ConfigHandler{svc: svc}
}

// GetPublic 获取公开配置值（无需认证）。
//
// GET /api/v1/public/configs/:key
// 公开键判定委托给 ConfigService.IsPublicKey，Handler 不再维护独立白名单。
func (h *ConfigHandler) GetPublic(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}
	if !h.svc.IsPublicKey(key) {
		response.Error(c, errcode.ErrNotFound, "配置不存在")
		return
	}

	val, err := h.svc.GetConfig(c.Request.Context(), key)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, val)
}

// Get 获取指定 key 的配置值。
//
// GET /api/v1/admin/configs/:key
func (h *ConfigHandler) Get(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}

	val, err := h.svc.GetConfig(c.Request.Context(), key)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, val)
}

// Update 更新或创建系统配置。
//
// PUT /api/v1/admin/configs/:key
//
// 使用 json.RawMessage 检查 "value" 键是否存在，
// 避免 binding:"required" 将 false/0/"" 等合法值误判为缺失。
func (h *ConfigHandler) Update(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.Error(c, errcode.ErrParam, "配置 key 不能为空")
		return
	}

	// 先读取原始 JSON 检查 "value" 键是否存在
	raw, err := c.GetRawData()
	if err != nil {
		response.Error(c, errcode.ErrParam, "读取请求体失败")
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		response.Error(c, errcode.ErrParam, "请求体不是合法 JSON")
		return
	}
	valRaw, ok := m["value"]
	if !ok {
		response.Error(c, errcode.ErrParam, "缺少 value 字段")
		return
	}

	// 反序列化 value 为任意类型
	var val interface{}
	if err := json.Unmarshal(valRaw, &val); err != nil {
		response.Error(c, errcode.ErrParam, "value 字段解析失败")
		return
	}

	updatedBy, _ := getCurrentUserID(c)
	if err := h.svc.UpdateConfig(c.Request.Context(), key, val, updatedBy); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// ComputeThresholds 计算置信度阈值分位数。
//
// POST /api/v1/admin/confidence/compute-thresholds
func (h *ConfigHandler) ComputeThresholds(c *gin.Context) {
	var req request.ComputeThresholdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败")
		return
	}

	result, err := h.svc.ComputeThresholds(c.Request.Context(), req.Days)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, result)
}

// =============================================================================
// 数据看板 DashboardHandler
// =============================================================================

// DashboardHandler 数据看板接口。
type DashboardHandler struct {
	svc *DashboardService
}

// NewDashboardHandler 创建 DashboardHandler 实例。
func NewDashboardHandler(svc *DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// GetStats 获取看板统计数据。
//
// GET /api/v1/admin/dashboard/stats
func (h *DashboardHandler) GetStats(c *gin.Context) {
	resp, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetTrends 获取趋势数据。
//
// GET /api/v1/admin/dashboard/trends
func (h *DashboardHandler) GetTrends(c *gin.Context) {
	var req request.TrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	resp, err := h.svc.GetTrends(c.Request.Context(), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// =============================================================================
// 站内消息 MessageHandler
// =============================================================================

// MessageHandler 站内消息接口。
type MessageHandler struct {
	svc *MessageService
}

// NewMessageHandler 创建 MessageHandler 实例。
func NewMessageHandler(svc *MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

// =============================================================================
// 门户端
// =============================================================================

// ListMessages 查询当前用户的消息列表，支持 is_read/type 过滤。
//
// GET /api/v1/portal/messages?is_read=true&type=ticket_supplement
func (h *MessageHandler) ListMessages(c *gin.Context) {
	userID, _ := getCurrentUserID(c)

	page, pageSize := parsePagination(c)

	// 解析可选过滤参数
	var filter MessageFilter
	if v := c.Query("is_read"); v != "" {
		b := v == "true" || v == "1"
		filter.IsRead = &b
	}
	filter.Type = c.Query("type")

	msgs, total, err := h.svc.ListMessages(c.Request.Context(), userID, page, pageSize, filter)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.SuccessWithPage(c, msgs, total, page, pageSize)
}

// MarkAsRead 标记消息为已读。
//
// PUT /api/v1/portal/messages/:id/read
// 校验消息归属（currentUserID），防止水平越权。
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的消息 ID")
		return
	}

	userID, _ := getCurrentUserID(c)
	count, err := h.svc.MarkAsReadAndCount(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, dto.MarkAsReadResponse{UnreadCount: count})
}

// MarkAllRead 标记当前用户所有消息为已读。
//
// PUT /api/v1/portal/messages/read-all
func (h *MessageHandler) MarkAllRead(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	affected, err := h.svc.MarkAllRead(c.Request.Context(), userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, dto.MarkAllReadResponse{Affected: affected})
}

// CountUnread 查询未读消息数。
//
// GET /api/v1/portal/messages/unread-count
func (h *MessageHandler) CountUnread(c *gin.Context) {
	userID, _ := getCurrentUserID(c)

	count, err := h.svc.CountUnread(c.Request.Context(), userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, dto.UnreadCountResponse{Count: count})
}
