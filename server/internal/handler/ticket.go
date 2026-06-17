// Package handler 实现 HTTP 请求处理。
//
// ticket.go 提供申告管理相关接口。
// Handler 层职责：参数解析、调用 Service、格式化响应。
// 状态机校验和业务规则在 Service 层完成。
package handler

import (
	"strconv"
	"strings"

	"opsmind/internal/dto/request"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// TicketHandler 申告管理接口。
type TicketHandler struct {
	svc *service.TicketService
}

// NewTicketHandler 创建 TicketHandler 实例。
func NewTicketHandler(svc *service.TicketService) *TicketHandler {
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
	if err := h.svc.CreateTicket(req, userID); err != nil {
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

	result, err := h.svc.ListByUser(userID, page, pageSize)
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
	if svcErr := h.svc.SupplementTicket(id, userID, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
}

// =============================================================================
// 后台管理
// =============================================================================

// ListAll 分页查询全部申告（支持按状态和紧急程度筛选）。
//
// GET /api/v1/admin/tickets
func (h *TicketHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePagination(c)
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	urgency, _ := strconv.Atoi(c.DefaultQuery("urgency", "0"))

	result, err := h.svc.ListAll(status, urgency, page, pageSize)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.SuccessWithPage(c, result.Tickets, result.Total, page, pageSize)
}

// GetDetail 获取申告详情。
//
// GET /api/v1/admin/tickets/:id  — 后台查看（不限所有权）
// GET /api/v1/portal/tickets/:id — 门户查看（仅限自己的申告）
func (h *TicketHandler) GetDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的申告 ID")
		return
	}

	// 门户端受限于所有权，后台端可查看全部
	userID, _ := getCurrentUserID(c)
	if strings.HasPrefix(c.FullPath(), "/api/v1/admin/") {
		userID = 0 // 后台不限制所有权
	}

	result, svcErr := h.svc.GetDetail(id, userID)
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
	if svcErr := h.svc.UpdateStatus(id, userID, req); svcErr != nil {
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
	if svcErr := h.svc.AddRecord(id, userID, req); svcErr != nil {
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
	if err := h.svc.CreateKnowledgeCandidate(id, body.KBID, userID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}
