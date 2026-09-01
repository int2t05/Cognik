// handler.go 提供申告管理相关 HTTP 接口。
//
// Handler 层职责：参数校验、调用 Service、格式化响应，不包含业务规则。
// parsePagination / getCurrentUserID / handleServiceError 为本领域 Handler 自用的本地副本，
// 与 handler/common.go 中的同名函数行为一致——领域包独立编译，不依赖 handler 包。
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

// parsePagination 从查询参数中解析分页参数（page, pageSize）。
//
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
//
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
//
// AppError 类型提取业务码，其他错误视为 500。
func handleServiceError(c *gin.Context, err error) {
	var appErr AppError
	if errors.As(err, &appErr) {
		response.Error(c, appErr.Code, appErr.Message)
		return
	}
	// 非 AppError 说明是未预期的内部错误，记录真实原因方便排查
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

	result, err := h.svc.ListByUser(c.Request.Context(), userID, page, pageSize)
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

	result, err := h.svc.ListAll(c.Request.Context(), status, page, pageSize)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.SuccessWithPage(c, result.Tickets, result.Total, page, pageSize)
}

// GetDetailAdmin 获取申告详情（后台——不限所有权）。
//
// GET /api/v1/admin/tickets/:id
// 为什么独立方法而非路由前缀判断：Handler 不应感知 URL 结构，
// 路由逻辑应留在 Router 层。
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
