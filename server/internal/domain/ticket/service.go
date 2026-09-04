// Package ticket 实现工单领域业务逻辑：CRUD、状态机转换、处理记录管理。
// 依赖通过消费者接口注入，避免跨领域循环依赖。
package ticket

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"cognos/internal/domain/system/audit"
	"cognos/internal/infra/runtime"
	"cognos/internal/shared/dto/request"
	"cognos/internal/shared/dto/response"
	"cognos/internal/shared/model"
	"cognos/internal/shared/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AppError 是 errcode.AppError 的类型别名，供本包内使用。
type AppError = errcode.AppError

// MessageNotifier 工单通知接口（消费者接口）。
type MessageNotifier interface {
	NotifySupplement(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error
	NotifyTicketResolved(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error
	NotifyTicketClosed(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error
	NotifyTicketOverdue(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error
}

// KnowledgeCandidateSaver 知识候选保存接口（消费者接口）。
type KnowledgeCandidateSaver interface {
	CreateArticle(ctx context.Context, req request.CreateArticleRequest, userID int64) (*model.KnowledgeArticle, error)
}

// FeedbackMarker 隐式反馈标记接口——工单创建后自动标记相关 AI 回答为"无帮助"。
type FeedbackMarker interface {
	MarkLastAssistantUnhelpful(ctx context.Context, sessionID int64) error
}

// TicketService 工单管理服务。
type TicketService struct {
	repo               *TicketRepo
	auditWriter        audit.AuditWriter
	txManager          runtime.TxManager
	msgSvc             MessageNotifier
	knowledgeCandidate KnowledgeCandidateSaver
	feedbackMarker     FeedbackMarker
}

// NewTicketService 创建 TicketService 实例。
func NewTicketService(repo *TicketRepo, auditWriter audit.AuditWriter, txManager runtime.TxManager, msgSvc MessageNotifier, knowledgeCandidate KnowledgeCandidateSaver, feedbackMarker FeedbackMarker) *TicketService {
	return &TicketService{repo: repo, auditWriter: auditWriter, txManager: txManager, msgSvc: msgSvc, knowledgeCandidate: knowledgeCandidate, feedbackMarker: feedbackMarker}
}

// =============================================================================
// CreateTicket
// =============================================================================

// CreateTicket 创建工单工单（status=Pending, source=Portal, ticket_no=TK-YYYYMMDD-NNNNNN）。
func (s *TicketService) CreateTicket(ctx context.Context, req request.CreateTicketRequest, userID int64) error {
	// 参数校验
	if strings.TrimSpace(req.Title) == "" {
		return AppError{Code: errcode.ErrParam, Message: "标题不能为空"}
	}
	if strings.TrimSpace(req.Description) == "" {
		return AppError{Code: errcode.ErrParam, Message: "描述不能为空"}
	}
	if strings.TrimSpace(req.ContactPhone) == "" {
		return AppError{Code: errcode.ErrParam, Message: "联系电话不能为空"}
	}

	// 生成唯一 ticket_no：日期 + crypto/rand 6 位随机数，唯一索引兜底。
	ticketNo, err := generateTicketNo()
	if err != nil {
		return AppError{Code: errcode.ErrUnknown, Message: "生成工单编号失败，请重试"}
	}

	// 序列化 Tags
	var tagsJSON datatypes.JSON
	if len(req.Tags) > 0 {
		tagsJSON = marshalTicketTags(req.Tags)
	}

	// 序列化 ChatContext（若提供）
	var chatCtxJSON datatypes.JSON
	if req.ChatContext != nil {
		raw, err := json.Marshal(req.ChatContext)
		if err != nil {
			return AppError{Code: errcode.ErrParam, Message: "序列化 chat_context 失败"}
		}
		chatCtxJSON = datatypes.JSON(raw)
	}

	ticket := &model.Ticket{
		TicketNo:     ticketNo,
		UserID:       userID,
		Title:        req.Title,
		Description:  req.Description,
		Tags:         tagsJSON,
		ContactPhone: req.ContactPhone,
		ContactEmail: req.ContactEmail,
		DeadlineAt:   req.DeadlineAt,
		ChatContext:  chatCtxJSON,
		Status:       model.TicketStatusPending,
		Source:       model.TicketSourcePortal,
	}

	if err := s.repo.Create(ctx, ticket); err != nil {
		return err
	}

	// 隐式反馈：带 ChatContext 时标记最后一条 AI 回答为"无帮助"，失败仅记日志。
	if req.ChatContext != nil && req.ChatContext.SessionID > 0 && s.feedbackMarker != nil {
		if err := s.feedbackMarker.MarkLastAssistantUnhelpful(ctx, req.ChatContext.SessionID); err != nil {
			slog.Warn("隐式反馈标记失败（工单已创建）", "session_id", req.ChatContext.SessionID, "ticket_no", ticketNo, "error", err)
		}
	}

	return nil
}

// =============================================================================
// UpdateTicket / SupplementTicket
// =============================================================================

// UpdateTicket 编辑工单（仅工单人，仅 Pending/Processing 状态，仅更新非空字段）。
func (s *TicketService) UpdateTicket(ctx context.Context, id int64, userID int64, req request.UpdateTicketRequest) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "工单不存在"}
		}
		return err
	}

	if ticket.UserID != userID {
		return AppError{Code: errcode.ErrForbidden, Message: "仅工单人可编辑"}
	}
	if ticket.Status != model.TicketStatusPending && ticket.Status != model.TicketStatusProcessing {
		return AppError{Code: errcode.ErrParam, Message: "仅待处理或处理中的工单可编辑"}
	}

	// 仅更新非空字段
	if req.Title != "" {
		ticket.Title = req.Title
	}
	if req.Description != "" {
		ticket.Description = req.Description
	}
	if req.ContactPhone != "" {
		ticket.ContactPhone = req.ContactPhone
	}
	if req.ContactEmail != "" {
		ticket.ContactEmail = req.ContactEmail
	}
	if len(req.Tags) > 0 {
		ticket.Tags = marshalTicketTags(req.Tags)
	}
	// DeadlineAt：nil 不更新，非 nil 设置（PATCH 语义，不支持清空）
	if req.DeadlineAt != nil {
		ticket.DeadlineAt = req.DeadlineAt
	}

	return s.repo.Update(ctx, ticket)
}

// SupplementTicket 补充工单信息（仅工单人，仅 NeedSupplement 状态，CAS 转 Processing，事务原子）。
func (s *TicketService) SupplementTicket(ctx context.Context, id int64, userID int64, req request.SupplementTicketRequest) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "工单不存在"}
		}
		return err
	}

	if ticket.UserID != userID {
		return AppError{Code: errcode.ErrForbidden, Message: "仅工单人可补充信息"}
	}

	if ticket.Status != model.TicketStatusNeedSupplement {
		return AppError{Code: errcode.ErrParam, Message: "仅需补充信息状态可补充"}
	}

	// 事务内原子执行：CreateRecord + UpdateStatus(CAS)
	return s.txManager.Transaction(ctx, func(tx *gorm.DB) error {
		txRepo := NewTicketRepo(tx)

		record := &model.TicketRecord{
			TicketID:   id,
			OperatorID: userID,
			Action:     model.TicketActionSupplement,
			Content:    req.Content,
		}
		if err := txRepo.CreateRecord(ctx, record); err != nil {
			return err
		}

		// CAS: 仅在 status=NeedSupplement 时更新为 Processing
		rows, err := txRepo.UpdateStatus(ctx, id, int(model.TicketStatusNeedSupplement), int(model.TicketStatusProcessing))
		if err != nil {
			return err
		}
		if rows == 0 {
			return AppError{Code: errcode.ErrParam, Message: "工单状态已变更，请刷新后重试"}
		}
		return nil
	})
}

// WithdrawTicket 用户撤回未处理工单（仅工单人、仅待处理态可撤回）。
// CAS: Pending→Withdrawn，创建撤回记录。
func (s *TicketService) WithdrawTicket(ctx context.Context, id int64, userID int64) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "工单不存在"}
		}
		return err
	}

	if ticket.UserID != userID {
		return AppError{Code: errcode.ErrForbidden, Message: "仅工单人可撤回"}
	}

	if ticket.Status != model.TicketStatusPending {
		return AppError{Code: errcode.ErrParam, Message: "仅待处理状态可撤回"}
	}

	return s.txManager.Transaction(ctx, func(tx *gorm.DB) error {
		txRepo := NewTicketRepo(tx)

		record := &model.TicketRecord{
			TicketID:   id,
			OperatorID: userID,
			Action:     model.TicketActionWithdraw,
			Content:    "用户撤回工单",
		}
		if err := txRepo.CreateRecord(ctx, record); err != nil {
			return err
		}

		// CAS: 仅在 status=Pending 时更新为 Withdrawn
		rows, err := txRepo.UpdateStatus(ctx, id, int(model.TicketStatusPending), int(model.TicketStatusWithdrawn))
		if err != nil {
			return err
		}
		if rows == 0 {
			return AppError{Code: errcode.ErrParam, Message: "工单状态已变更，请刷新后重试"}
		}
		return nil
	})
}

// =============================================================================
// UpdateStatus
// =============================================================================

// UpdateStatus 执行工单状态转换（CAS 防并发，每次转换创建 TicketRecord）。
// Pending→Processing / Processing→NeedSupplement(count<3) / Processing→Resolved / 非Closed/Resolved→Closed
func (s *TicketService) UpdateStatus(ctx context.Context, id int64, operatorID int64, req request.UpdateTicketStatusRequest) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "工单不存在"}
		}
		return err
	}

	var newStatus int16
	var recordAction string

	switch req.Action {
	case model.TicketActionStart:
		if ticket.Status != model.TicketStatusPending {
			return AppError{Code: errcode.ErrParam, Message: "仅待处理状态可开始处理"}
		}
		newStatus = model.TicketStatusProcessing
		recordAction = model.TicketActionStart

	case model.TicketActionRequestInfo:
		if ticket.Status != model.TicketStatusProcessing {
			return AppError{Code: errcode.ErrParam, Message: "仅处理中状态可请求补充信息"}
		}
		// 原子自增 supplement_count，WHERE supplement_count < 3 保证并发安全
		ok, err := s.repo.IncrementSupplementCount(ctx, id)
		if err != nil {
			return err
		}
		if !ok {
			return AppError{Code: errcode.ErrParam, Message: "补充信息次数已达上限（3次）"}
		}
		newStatus = model.TicketStatusNeedSupplement
		recordAction = model.TicketActionRequestInfo

	case model.TicketActionResolve:
		if ticket.Status != model.TicketStatusProcessing {
			return AppError{Code: errcode.ErrParam, Message: "仅处理中状态可解决"}
		}
		newStatus = model.TicketStatusResolved
		recordAction = model.TicketActionResolve

	case model.TicketActionClose:
		// 已关闭不允许重复关闭；已解决不允许回退为关闭
		if ticket.Status == model.TicketStatusClosed {
			return AppError{Code: errcode.ErrParam, Message: "工单已关闭，无需重复操作"}
		}
		if ticket.Status == model.TicketStatusResolved {
			return AppError{Code: errcode.ErrParam, Message: "已解决的工单不允许关闭"}
		}
		newStatus = model.TicketStatusClosed
		recordAction = model.TicketActionClose

	default:
		return AppError{Code: errcode.ErrParam, Message: "不支持的操作类型: " + req.Action}
	}

	// 事务内原子执行：UpdateStatus(CAS) + CreateRecord + 审计日志
	err = s.txManager.Transaction(ctx, func(tx *gorm.DB) error {
		txRepo := NewTicketRepo(tx)

		// CAS 防并发
		rows, err := txRepo.UpdateStatus(ctx, id, int(ticket.Status), int(newStatus))
		if err != nil {
			return err
		}
		if rows == 0 {
			return AppError{Code: errcode.ErrParam, Message: "工单状态已变更，请刷新后重试"}
		}

		record := &model.TicketRecord{
			TicketID:   id,
			OperatorID: operatorID,
			Action:     recordAction,
			Content:    req.Result,
		}
		if err := txRepo.CreateRecord(ctx, record); err != nil {
			return err
		}
		// 审计日志（同事务）
		if err := s.auditWriter.WriteWithTx(ctx, tx, operatorID, "ticket."+req.Action, "ticket", id, ""); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// request_info 成功后同步通知工单人
	if s.msgSvc != nil {
		switch recordAction {
		case model.TicketActionRequestInfo:
			if err := s.msgSvc.NotifySupplement(ctx, id, ticket.UserID, ticket.Title); err != nil {
				slog.Warn("补充信息通知失败", "ticket_id", id, "error", err)
			}
		case model.TicketActionResolve:
			if err := s.msgSvc.NotifyTicketResolved(ctx, id, ticket.UserID, ticket.Title); err != nil {
				slog.Warn("已解决通知失败", "ticket_id", id, "error", err)
			}
		case model.TicketActionClose:
			if err := s.msgSvc.NotifyTicketClosed(ctx, id, ticket.UserID, ticket.Title); err != nil {
				slog.Warn("已关闭通知失败", "ticket_id", id, "error", err)
			}
		}
	}

	slog.Info("工单状态变更", "ticket_id", id, "action", recordAction,
		"from", ticket.Status, "to", newStatus, "operator", operatorID)
	return nil
}

// =============================================================================
// AddRecord
// =============================================================================

// AddRecord 添加处理记录（不影响状态，action 白名单校验，detail 校验 JSON）。
func (s *TicketService) AddRecord(ctx context.Context, id int64, operatorID int64, req request.CreateTicketRecordRequest) error {
	if !isValidRecordAction(req.Action) {
		return AppError{Code: errcode.ErrParam, Message: "不支持的记录类型: " + req.Action}
	}

	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "工单不存在"}
		}
		return err
	}

	var detailJSON datatypes.JSON
	if req.Detail != "" {
		if !isValidJSON(req.Detail) {
			return AppError{Code: errcode.ErrParam, Message: "detail 不是合法的 JSON"}
		}
		detailJSON = datatypes.JSON(req.Detail)
	}

	record := &model.TicketRecord{
		TicketID:   id,
		OperatorID: operatorID,
		Action:     req.Action,
		Content:    req.Content,
		Detail:     detailJSON,
	}
	return s.repo.CreateRecord(ctx, record)
}

// =============================================================================
// ListByUser / ListAll / GetDetail
// =============================================================================

// ListByUser 分页查询当前用户的工单列表（status=-1 不过滤）。
func (s *TicketService) ListByUser(ctx context.Context, userID int64, page, pageSize int, keyword string, status int) (*response.TicketListResponse, error) {
	tickets, total, err := s.repo.ListByUser(ctx, userID, page, pageSize, keyword, status)
	if err != nil {
		return nil, err
	}

	items := make([]response.TicketItem, len(tickets))
	for i, t := range tickets {
		items[i] = toTicketItem(&t)
	}

	return &response.TicketListResponse{
		Tickets: items,
		Total:   total,
	}, nil
}

// ListAll 分页查询全部工单（status=-1 不过滤）。
func (s *TicketService) ListAll(ctx context.Context, status, page, pageSize int, keyword string) (*response.TicketListResponse, error) {
	tickets, total, err := s.repo.ListAll(ctx, status, page, pageSize, keyword)
	if err != nil {
		return nil, err
	}

	items := make([]response.TicketItem, len(tickets))
	for i, t := range tickets {
		items[i] = toTicketItem(&t)
	}

	return &response.TicketListResponse{
		Tickets: items,
		Total:   total,
	}, nil
}

// GetDetail 获取工单详情（含处理记录时间线）。userID>0 做所有权检查，=0 跳过（后台）。
func (s *TicketService) GetDetail(ctx context.Context, id int64, userID int64) (*response.TicketDetailResponse, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "工单不存在"}
		}
		return nil, err
	}

	if userID > 0 && ticket.UserID != userID {
		return nil, AppError{Code: errcode.ErrForbidden, Message: "无权查看此工单"}
	}

	records := make([]response.TicketRecordItem, len(ticket.TicketRecords))
	for i, r := range ticket.TicketRecords {
		records[i] = response.TicketRecordItem{
			ID:         r.ID,
			TicketID:   r.TicketID,
			OperatorID: r.OperatorID,
			Action:     r.Action,
			Content:    r.Content,
			Detail:     string(r.Detail),
			CreatedAt:  r.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	detail := &response.TicketDetailResponse{
		TicketItem: toTicketItem(ticket),
	}
	detail.Description = ticket.Description
	detail.ContactEmail = ticket.ContactEmail
	detail.Source = ticket.Source
	detail.Records = records

	// 反序列化标签
	if len(ticket.Tags) > 0 {
		detail.Tags = unmarshalTicketTags(ticket.Tags)
	}

	return detail, nil
}

// =============================================================================
// 辅助函数
// =============================================================================

// toTicketItem 将 model.Ticket 转换为 TicketItem。
func toTicketItem(t *model.Ticket) response.TicketItem {
	submitterName := ""
	if t.User.ID != 0 {
		submitterName = t.User.RealName
	}

	return response.TicketItem{
		ID:              t.ID,
		TicketNo:        t.TicketNo,
		UserID:          t.UserID,
		SubmitterName:   submitterName,
		Title:           t.Title,
		Tags:            unmarshalTicketTags(t.Tags),
		ContactPhone:    t.ContactPhone,
		Status:          t.Status,
		StatusText:      model.TicketStatusText(t.Status),
		SupplementCount: t.SupplementCount,
		DeadlineAt:      formatTimePtr(t.DeadlineAt),
		CreatedAt:       t.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:       t.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// formatTimePtr 将可空时间指针格式化为字符串指针，nil 返回 nil。
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02 15:04:05")
	return &s
}

// marshalTicketTags 将标签切片序列化为 JSON。
func marshalTicketTags(items []string) datatypes.JSON {
	if len(items) == 0 {
		return datatypes.JSON("[]")
	}
	data, err := json.Marshal(items)
	if err != nil {
		return datatypes.JSON("[]")
	}
	return datatypes.JSON(data)
}

// unmarshalTicketTags 将 JSON 反序列化为标签切片。
func unmarshalTicketTags(data datatypes.JSON) []string {
	if len(data) == 0 {
		return nil
	}
	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

// =============================================================================
// AutoClose（定时任务 — Scheduler 调用）
// =============================================================================

// AutoClose 自动关闭超期工单（Scheduler 调用，事务内 UPDATE + TicketRecord）。
func (s *TicketService) AutoClose(ctx context.Context, olderThan time.Time) (int64, error) {
	var closedCount int64

	err := s.txManager.Transaction(ctx, func(tx *gorm.DB) error {
		txRepo := NewTicketRepo(tx)

		ids, err := txRepo.AutoCloseTickets(ctx, olderThan)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		now := time.Now()
		for _, id := range ids {
			if err := tx.Create(&model.TicketRecord{
				TicketID: id, OperatorID: 0, Action: "auto_close",
				Content: "系统自动关闭：工单超过 7 天未处理", CreatedAt: now,
			}).Error; err != nil {
				slog.Warn("auto_close 创建记录失败，跳过该工单", "ticket_id", id, "error", err)
				continue
			}
			if err := s.auditWriter.WriteWithTx(ctx, tx, 0, "ticket.auto_close", "ticket", id, ""); err != nil {
				continue
			}
		}

		closedCount = int64(len(ids))
		return nil
	})

	return closedCount, err
}

// ScanOverdueTickets 扫描已超时未完结的工单并逐条发送超时通知（Scheduler 调用）。
// 返回处理的超时工单条数。同一工单可能被重复扫描——由消息去重或人工处理后自愈，此处不持久化已通知标记。
func (s *TicketService) ScanOverdueTickets(ctx context.Context, now time.Time) (int64, error) {
	tickets, err := s.repo.ListOverdue(ctx, now)
	if err != nil {
		return 0, err
	}
	var notified int64
	for _, t := range tickets {
		if err := s.NotifyTicketOverdue(ctx, t.ID, t.UserID, t.Title); err != nil {
			slog.Warn("超时通知发送失败", "ticket_id", t.ID, "error", err)
			continue
		}
		notified++
	}
	return notified, nil
}

// =============================================================================
// CreateKnowledgeCandidate
// =============================================================================

// CreateKnowledgeCandidate 从工单内容生成知识库候选文章。
func (s *TicketService) CreateKnowledgeCandidate(ctx context.Context, id int64, kbID int64, userID int64) error {
	detail, err := s.GetDetail(ctx, id, 0)
	if err != nil {
		return err
	}

	// 结构化知识候选：标题 / 详细描述 / 解决方案（待人工补充），标签与工单互通
	content := fmt.Sprintf("## 标题\n%s\n\n## 详细描述\n%s\n\n## 解决方案\n> 请根据实际情况补充解决方案",
		detail.Title, detail.Description)
	articleReq := request.CreateArticleRequest{
		KBID:    kbID,
		Title:   "工单经验 - " + detail.Title,
		Content: content,
		Tags:    detail.Tags,
	}

	if s.knowledgeCandidate == nil {
		return AppError{Code: errcode.ErrUnknown, Message: "知识库服务未初始化"}
	}
	if _, err := s.knowledgeCandidate.CreateArticle(ctx, articleReq, userID); err != nil {
		return err
	}

	slog.Info("从工单创建知识候选", "ticket_id", id, "kb_id", kbID, "operator", userID)
	return nil
}

// =============================================================================
// 工具函数
// =============================================================================

// generateTicketNo 生成工单编号 TK-YYYYMMDD-NNNNNN（crypto/rand 6 位随机数）。
func generateTicketNo() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("TK-%s-%06d", time.Now().Format("20060102"), n.Int64()), nil
}

// isValidJSON 校验字符串是否为合法 JSON。
func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// validRecordActions 处理记录 action 白名单。
var validRecordActions = map[string]bool{
	"note":     true,
	"callback": true,
	"escalate": true,
}

func isValidRecordAction(action string) bool {
	return validRecordActions[action]
}

// BatchDelete 批量删除工单（含关联处理记录）。
func (s *TicketService) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	return s.repo.BatchDelete(ctx, ids)
}

// BatchCloseResult 批量关闭单项结果。
type BatchCloseResult struct {
	ID       int64  `json:"id"`
	Success  bool   `json:"success"`
	ErrorMsg string `json:"error_msg"`
}

// BatchClose 批量关闭工单，逐条复用单条 close 逻辑（CAS+Record+审计+消息），部分失败不影响其他。
func (s *TicketService) BatchClose(ctx context.Context, ids []int64, operatorID int64) []BatchCloseResult {
	results := make([]BatchCloseResult, len(ids))
	for i, id := range ids {
		err := s.UpdateStatus(ctx, id, operatorID, request.UpdateTicketStatusRequest{Action: model.TicketActionClose})
		results[i] = BatchCloseResult{ID: id}
		if err != nil {
			results[i].ErrorMsg = errcode.ExtractErrMsg(err)
			continue
		}
		results[i].Success = true
	}
	return results
}

// NotifyTicketOverdue 通知工单处理超时（调度器调用，通知工单创建人）。
func (s *TicketService) NotifyTicketOverdue(ctx context.Context, ticketID int64, userID int64, ticketTitle string) error {
	if s.msgSvc != nil {
		return s.msgSvc.NotifyTicketOverdue(ctx, ticketID, userID, ticketTitle)
	}
	return nil
}
