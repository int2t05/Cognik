// handler.go 申告管理 HTTP 接口（参数校验、调用 Service、格式化响应）。
package ticket

import (
	"errors"
	"log/slog"
	"strconv"

	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// Handler 共享工具
// =============================================================================

// parsePagination 解析分页参数（page 默认 1，pageSize 默认 10，上限 100）。
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

// getCurrentUserID 从 Gin context 获取当前用户 ID。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// handleServiceError 统一处理 Service 错误（AppError 提取业务码，其余 500）。
func handleServiceError(c *gin.Context, err error) {
	var appErr AppError
	if errors.As(err, &appErr) {
		response.Error(c, appErr.Code, appErr.Message)
		return
	}
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	response.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// =============================================================================
// TicketHandler
// =============================================================================

// TicketHandler 申告管理接口。
type TicketHandler struct {
	svc *TicketService
}

// NewTicketHandler 创建 TicketHandler 实例。
func NewTicketHandler(svc *TicketService) *TicketHandler {
	return &TicketHandler{svc: svc}
}

// =============================================================================
// 门户端
// =============================================================================

// CreateTicket 创建申告。
//
// POST /api/v1/portal/tickets
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	var req request.CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if err := h.svc.CreateTicket(c.Request.Context(), req, userID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// ListByUser 查询当前用户的申告列表。
//
// GET /api/v1/portal/tickets
func (h *TicketHandler) ListByUser(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	page, pageSize := parsePagination(c)
	keyword := c.DefaultQuery("keyword", "")

	result, err := h.svc.ListByUser(c.Request.Context(), userID, page, pageSize, keyword)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.SuccessWithPage(c, result.Tickets, result.Total, page, pageSize)
}

// SupplementTicket 补充申告信息。
//
// PATCH /api/v1/portal/tickets/:id/supplement
func (h *TicketHandler) SupplementTicket(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	var req request.SupplementTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.SupplementTicket(c.Request.Context(), id, userID, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// UpdateTicket 编辑申告（仅申告人可操作）。
//
// PATCH /api/v1/portal/tickets/:id
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	var req request.UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.UpdateTicket(c.Request.Context(), id, userID, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// =============================================================================
// 后台管理
// =============================================================================

// ListAll 分页查询全部申告（支持按状态筛选）。
//
// GET /api/v1/admin/tickets
func (h *TicketHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePagination(c)
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	keyword := c.DefaultQuery("keyword", "")

	result, err := h.svc.ListAll(c.Request.Context(), status, page, pageSize, keyword)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.SuccessWithPage(c, result.Tickets, result.Total, page, pageSize)
}

// GetDetailAdmin 获取申告详情（后台，不限所有权）。
func (h *TicketHandler) GetDetailAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	result, svcErr := h.svc.GetDetail(c.Request.Context(), id, 0)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, result)
}

// GetDetailPortal 获取申告详情（门户——仅限自己的申告）。
//
// GET /api/v1/portal/tickets/:id
func (h *TicketHandler) GetDetailPortal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	userID, _ := getCurrentUserID(c)
	result, svcErr := h.svc.GetDetail(c.Request.Context(), id, userID)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, result)
}

// UpdateStatus 更新申告状态（状态机转换）。
//
// PATCH /api/v1/admin/tickets/:id/status
func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	var req request.UpdateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.UpdateStatus(c.Request.Context(), id, userID, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// AddRecord 添加处理记录（不影响状态）。
//
// POST /api/v1/admin/tickets/:id/records
func (h *TicketHandler) AddRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	var req request.CreateTicketRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if svcErr := h.svc.AddRecord(c.Request.Context(), id, userID, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// =============================================================================
// 知识库候选
// =============================================================================

// CreateKnowledgeCandidate 从申告内容生成知识库候选条目。
//
// POST /api/v1/admin/tickets/:id/knowledge-candidate
func (h *TicketHandler) CreateKnowledgeCandidate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	var body struct {
		KBID int64 `json:"kb_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if err := h.svc.CreateKnowledgeCandidate(c.Request.Context(), id, body.KBID, userID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// BatchDelete 批量删除申告。
//
// POST /api/v1/admin/tickets/batch-delete
func (h *TicketHandler) BatchDelete(c *gin.Context) {
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

// BatchClose 批量关闭申告，逐条复用单条 close 状态机，部分失败不影响其他。
//
// POST /api/v1/admin/tickets/batch-close
func (h *TicketHandler) BatchClose(c *gin.Context) {
	var req request.BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}
	userID, _ := getCurrentUserID(c)
	results := h.svc.BatchClose(c.Request.Context(), req.IDs, userID)
	response.Success(c, map[string]any{"results": results})
}
