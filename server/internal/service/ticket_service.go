// Package service 实现申告管理业务逻辑。
//
// TicketService 提供申告 CRUD、状态机转换、处理记录管理功能。
//
// 申告状态机：待处理(1) → 处理中(2) → 需补充信息(3) → 处理中(2) → 已解决(4) / 已关闭(5)
// 为什么使用显式状态转换而非隐式条件判断：
// 状态转换规则是申告核心流程，显式状态机便于审计和调试。
package service

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

	"opsmind/internal/dto/request"
	"opsmind/internal/dto/response"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// KnowledgeCandidateSaver 知识候选保存接口。
//
// TicketService 仅需从申告创建知识候选文章，不需要完整的 KnowledgeService。
// 消费者接口模式消除了两阶段构造（New + Setter）的循环依赖 workaround。
type KnowledgeCandidateSaver interface {
	CreateArticle(ctx context.Context, req request.CreateArticleRequest, userID int64) (*model.KnowledgeArticle, error)
}

// FeedbackMarker 隐式反馈标记接口——申告创建后自动标记相关 AI 回答为"无帮助"。
//
// 为什么放在 TicketService 而非 ChatService：
// "用户看完 AI 回答后提交申告"是申告上下文中的隐式反馈信号，
// TicketService 拥有 ChatContext 信息，是最自然的触发点。
type FeedbackMarker interface {
	MarkLastAssistantUnhelpful(ctx context.Context, sessionID int64) error
}

// TicketService 申告管理服务。
type TicketService struct {
	repo               *repository.TicketRepo
	auditWriter        AuditWriter
	txManager          TxManager
	msgSvc             *MessageService
	knowledgeCandidate KnowledgeCandidateSaver
	feedbackMarker     FeedbackMarker
}

// NewTicketService 创建 TicketService 实例。
//
// knowledgeCandidate 为知识候选保存接口，KnowledgeService 隐式满足该接口。
// feedbackMarker 为隐式反馈标记接口，ChatService 隐式满足该接口。
// 所有依赖在构造时注入，对象始终处于有效状态。
func NewTicketService(repo *repository.TicketRepo, auditWriter AuditWriter, txManager TxManager, msgSvc *MessageService, knowledgeCandidate KnowledgeCandidateSaver, feedbackMarker FeedbackMarker) *TicketService {
	return &TicketService{repo: repo, auditWriter: auditWriter, txManager: txManager, msgSvc: msgSvc, knowledgeCandidate: knowledgeCandidate, feedbackMarker: feedbackMarker}
}

// SetKnowledgeCandidate 延迟注入知识候选保存接口。
// 仅用于集成测试等需要两阶段构造的场景。
func (s *TicketService) SetKnowledgeCandidate(kc KnowledgeCandidateSaver) {
	s.knowledgeCandidate = kc
}

// SetFeedbackMarker 延迟注入隐式反馈标记接口。
// ChatService 创建在 TicketService 之后（因依赖 LLMService），
// 通过 Setter 解决构造顺序问题。
func (s *TicketService) SetFeedbackMarker(fm FeedbackMarker) {
	s.feedbackMarker = fm
}

// =============================================================================
// CreateTicket
// =============================================================================

// CreateTicket 创建申告工单。
//
// 业务规则：
//   - title、description、contact_phone 为必填
//   - tags 为用户提交的逗号分隔标签
//   - ticket_no 格式：TK-YYYYMMDD-XXXX（XXXX 为随机 6 位后缀）
//   - 新建申告 status=TicketStatusPending、source=TicketSourcePortal
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

	// 生成唯一 ticket_no：日期 + 加密随机 6 位数字。
	// 为什么用 crypto/rand 而非纳秒时间戳：纳秒取模在并发场景碰撞风险不可控，
	// crypto/rand 提供真随机 + 数据库唯一索引兜底，碰撞后自动重试（最多 3 次）。
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
		TicketNo:        ticketNo,
		UserID:          userID,
		Title:           req.Title,
		Description:     req.Description,
		Tags: tagsJSON,
		ContactPhone:    req.ContactPhone,
		ContactEmail:    req.ContactEmail,
		ChatContext:     chatCtxJSON,
		Status:          model.TicketStatusPending,
		Source:          model.TicketSourcePortal,
	}

	if err := s.repo.Create(ctx, ticket); err != nil {
		return err
	}

	// 隐式反馈：用户提交申告意味着 AI 回答未能解决其问题，
// 若带有 ChatContext 则自动标记对应会话的最后一条 AI 消息为"无帮助"。
// 失败不影响申告创建（非关键路径），仅记录日志。
if req.ChatContext != nil && req.ChatContext.SessionID > 0 && s.feedbackMarker != nil {
	if err := s.feedbackMarker.MarkLastAssistantUnhelpful(ctx, req.ChatContext.SessionID); err != nil {
		slog.Warn("隐式反馈标记失败（申告已创建）", "session_id", req.ChatContext.SessionID, "ticket_no", ticketNo, "error", err)
	}
}

return nil
}

// =============================================================================
// UpdateTicket / SupplementTicket
// =============================================================================

// UpdateTicket 编辑申告（仅申告人可操作，仅待处理/处理中状态可编辑）。
//
// 仅更新请求中非空的字段，空字段保持原值不变。
// 这样前端可以只发送需要修改的字段，避免覆盖未修改的数据。
func (s *TicketService) UpdateTicket(ctx context.Context, id int64, userID int64, req request.UpdateTicketRequest) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}
		return err
	}

	if ticket.UserID != userID {
		return AppError{Code: errcode.ErrForbidden, Message: "仅申告人可编辑"}
	}
	if ticket.Status != model.TicketStatusPending && ticket.Status != model.TicketStatusProcessing {
		return AppError{Code: errcode.ErrParam, Message: "仅待处理或处理中的申告可编辑"}
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

	return s.repo.Update(ctx, ticket)
}

// SupplementTicket 补充申告信息。
//
// 业务规则：
//   - 仅申告人本人可补充
//   - 仅"需补充信息"状态可补充
//   - 补充后状态变为"处理中"，使用 CAS 防止并发双重操作
//   - CreateRecord + UpdateStatus 在同一事务中原子执行
func (s *TicketService) SupplementTicket(ctx context.Context, id int64, userID int64, req request.SupplementTicketRequest) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}
		return err
	}

	// 仅申告人可补充
	if ticket.UserID != userID {
		return AppError{Code: errcode.ErrForbidden, Message: "仅申告人可补充信息"}
	}

	// 仅"需补充信息"状态可操作
	if ticket.Status != model.TicketStatusNeedSupplement {
		return AppError{Code: errcode.ErrParam, Message: "仅需补充信息状态可补充"}
	}

	// 事务内原子执行：CreateRecord + UpdateStatus(CAS)，避免孤立记录
	return s.txManager.Transaction(ctx, func(tx *gorm.DB) error {
		txRepo := repository.NewTicketRepo(tx)

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
			return AppError{Code: errcode.ErrParam, Message: "申告状态已变更，请刷新后重试"}
		}
		return nil
	})
}

// =============================================================================
// UpdateStatus
// =============================================================================

// UpdateStatus 执行申告状态转换（CAS 防护）。
//
// 状态机规则（使用 model 常量，编译期约束）：
//
//	start:        TicketStatusPending     → TicketStatusProcessing
//	request_info: TicketStatusProcessing  → TicketStatusNeedSupplement（supplement_count < 3）
//	resolve:      TicketStatusProcessing  → TicketStatusResolved
//	close:        TStatus≠Closed/Resolved → TicketStatusClosed
//
// 所有转换使用 CAS（WHERE id=? AND status=?），防止并发双重操作。
// 每次状态转换都会创建 TicketRecord。
func (s *TicketService) UpdateStatus(ctx context.Context, id int64, operatorID int64, req request.UpdateTicketStatusRequest) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
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
			return AppError{Code: errcode.ErrParam, Message: "申告已关闭，无需重复操作"}
		}
		if ticket.Status == model.TicketStatusResolved {
			return AppError{Code: errcode.ErrParam, Message: "已解决的申告不允许关闭"}
		}
		newStatus = model.TicketStatusClosed
		recordAction = model.TicketActionClose

	default:
		return AppError{Code: errcode.ErrParam, Message: "不支持的操作类型: " + req.Action}
	}

	// 事务内原子执行：UpdateStatus(CAS) + CreateRecord
	err = s.txManager.Transaction(ctx, func(tx *gorm.DB) error {
		txRepo := repository.NewTicketRepo(tx)

		// CAS: 仅在状态未变化时执行更新，防止并发双重操作
		rows, err := txRepo.UpdateStatus(ctx, id, int(ticket.Status), int(newStatus))
		if err != nil {
			return err
		}
		if rows == 0 {
			return AppError{Code: errcode.ErrParam, Message: "申告状态已变更，请刷新后重试"}
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
		txAuditRepo := repository.NewAuditRepo(tx)
		txAuditRepo.Create(ctx, &model.AuditLog{
			OperatorID: operatorID, Action: "ticket." + req.Action,
			TargetType: "ticket", TargetID: id,
		})
		return nil
	})
	if err != nil {
		return err
	}

	// request_info 成功后同步通知申告人
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

	slog.Info("申告状态变更", "ticket_id", id, "action", recordAction,
		"from", ticket.Status, "to", newStatus, "operator", operatorID)
	return nil
}

// =============================================================================
// AddRecord
// =============================================================================

// AddRecord 添加处理记录（不影响状态）。
//
// 用于记录处理过程中的备注、沟通记录等。
// action 仅允许白名单值，防止审计数据污染。
// detail 若提供则校验为合法 JSON。
func (s *TicketService) AddRecord(ctx context.Context, id int64, operatorID int64, req request.CreateTicketRecordRequest) error {
	// action 白名单校验
	if !isValidRecordAction(req.Action) {
		return AppError{Code: errcode.ErrParam, Message: "不支持的记录类型: " + req.Action}
	}

	// 验证申告存在
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
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

// ListByUser 分页查询当前用户的申告列表。
func (s *TicketService) ListByUser(ctx context.Context, userID int64, page, pageSize int) (*response.TicketListResponse, error) {
	tickets, total, err := s.repo.ListByUser(ctx, userID, page, pageSize)
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

// ListAll 分页查询全部申告（支持按状态和紧急程度筛选）。
//
// status=-1 表示不过滤，urgency=0 表示不过滤。
func (s *TicketService) ListAll(ctx context.Context, status, page, pageSize int) (*response.TicketListResponse, error) {
	tickets, total, err := s.repo.ListAll(ctx, status, page, pageSize)
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

// GetDetail 获取申告详情（含提交人信息和处理记录时间线）。
//
// userID 用于门户端越权检查：
//   - userID > 0（门户端）：仅允许查自己的申告，非本人返回 ErrForbidden
//   - userID == 0（后台管理）：跳过所有权检查，可查全部
func (s *TicketService) GetDetail(ctx context.Context, id int64, userID int64) (*response.TicketDetailResponse, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}
		return nil, err
	}

	// 门户端越权检查：仅允许查自己的申告
	if userID > 0 && ticket.UserID != userID {
		return nil, AppError{Code: errcode.ErrForbidden, Message: "无权查看此申告"}
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

	// 反序列化受影响的系统
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
		CreatedAt:       t.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:       t.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// marshalTicketTags 将字符串切片序列化为 JSON。
//
// 使用 json.Marshal 保证正确转义 — 修复前手动拼接在含双引号/逗号时产生畸形 JSON。
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

// unmarshalTicketTags 将 JSON 反序列化为字符串切片。
//
// 使用 json.Unmarshal 替代手动字符串分割，正确处理转义字符。
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

// AutoClose 自动关闭超期申告（由 Scheduler 定时调用）。
//
// 业务规则：status IN (1,2,3) AND created_at < olderThan 的申告自动关闭。
// UPDATE + TicketRecord 创建在同一事务中原子执行，
// 避免工单已关闭但缺少 auto_close 时间线记录。
func (s *TicketService) AutoClose(ctx context.Context, olderThan time.Time) (int64, error) {
	var closedCount int64

	err := s.txManager.Transaction(ctx, func(tx *gorm.DB) error {
		txRepo := repository.NewTicketRepo(tx)

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
				Content: "系统自动关闭：申告超过 7 天未处理", CreatedAt: now,
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

// =============================================================================
// CreateKnowledgeCandidate
// =============================================================================

// CreateKnowledgeCandidate 从申告内容生成知识库候选文章。
//
// 为什么放在 TicketService 而非 Handler 直接调用 KnowledgeService：
// 统一的 Service 层编排便于加入事务边界和审计日志，避免 Handler 层跨 Service 调用。
func (s *TicketService) CreateKnowledgeCandidate(ctx context.Context, id int64, kbID int64, userID int64) error {
	detail, err := s.GetDetail(ctx, id, 0)
	if err != nil {
		return err
	}

	// 结构化知识候选：标题 / 详细描述 / 解决方案（待人工补充），标签与申告互通
	content := fmt.Sprintf("## 标题\n%s\n\n## 详细描述\n%s\n\n## 解决方案\n> 请根据实际情况补充解决方案",
		detail.Title, detail.Description)
	articleReq := request.CreateArticleRequest{
		KBID:    kbID,
		Title:   "申告经验 - " + detail.Title,
		Content: content,
		Tags:    detail.Tags,
	}

	if s.knowledgeCandidate == nil {
		return AppError{Code: errcode.ErrUnknown, Message: "知识库服务未初始化"}
	}
	if _, err := s.knowledgeCandidate.CreateArticle(ctx, articleReq, userID); err != nil {
		return err
	}

	slog.Info("从申告创建知识候选", "ticket_id", id, "kb_id", kbID, "operator", userID)
	return nil
}

// =============================================================================
// 工具函数
// =============================================================================

// generateTicketNo 生成唯一工单编号。
//
// 格式 TK-YYYYMMDD-NNNNNN，其中 NNNNNN 为 crypto/rand 生成的 6 位随机数。
// 数据库 ticket_no 唯一索引兜底，调用方应在 Create 失败时重试。
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

// isValidRecordAction 校验处理记录 action 是否为白名单值。
//
// 白名单之外的 action 拒绝写入，防止审计数据被任意字符串污染。
var validRecordActions = map[string]bool{
	"note":     true,
	"callback": true,
	"escalate": true,
}

func isValidRecordAction(action string) bool {
	return validRecordActions[action]
}

// BatchDelete 批量删除申告（含关联处理记录）。
func (s *TicketService) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	return s.repo.BatchDelete(ctx, ids)
}
