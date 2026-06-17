// Package service 实现申告管理业务逻辑。
//
// TicketService 提供申告 CRUD、状态机转换、处理记录管理功能。
//
// 申告状态机：待处理(1) → 处理中(2) → 需补充信息(3) → 处理中(2) → 已解决(4) / 已关闭(5)
// 为什么使用显式状态转换而非隐式条件判断：
// 状态转换规则是申告核心流程，显式状态机便于审计和调试。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// TicketService 申告管理服务。
type TicketService struct {
	repo      *repository.TicketRepo
	txManager TxManager
}

// NewTicketService 创建 TicketService 实例。
func NewTicketService(repo *repository.TicketRepo, txManager TxManager) *TicketService {
	return &TicketService{repo: repo, txManager: txManager}
}

// =============================================================================
// CreateTicket
// =============================================================================

// CreateTicket 创建申告工单。
//
// 业务规则：
//   - title、description、contact_phone 为必填
//   - urgency 必须为 1（低）、2（中）、3（高）
//   - ticket_no 格式：TK-YYYYMMDD-XXXX（XXXX 为随机 4 位后缀）
//   - 新建申告 status=1（待处理）、source=1（门户提交）
func (s *TicketService) CreateTicket(req request.CreateTicketRequest, userID int64) error {
	// TODO(service/ticket): 使用 model.TicketUrgency* 等常量替代裸数字 1/2/3。
	// 状态、紧急程度、来源散落裸值会让后续改枚举时难以全局追踪。
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
	if req.Urgency < 1 || req.Urgency > 3 {
		return AppError{Code: errcode.ErrParam, Message: "紧急程度必须为 1-3"}
	}

	// 生成唯一 ticket_no：日期 + 纳秒时间戳后6位
	// 相比 rand.Intn(10000)（仅 10000 种组合），纳秒时间戳提供百万级组合，
	// 结合日期前缀，碰撞概率极低。后续可升级为雪花算法或 DB 序列。
	// TODO(service/ticket): ticket_no 仍可能在高并发或多实例同纳秒场景碰撞。
	// 建议使用数据库序列/唯一索引重试/雪花 ID，确保唯一性失败时可自动重试。
	now := time.Now()
	datePart := now.Format("20060102")
	suffix := fmt.Sprintf("%06d", now.UnixNano()%1000000)
	ticketNo := fmt.Sprintf("TK-%s-%s", datePart, suffix)

	// 序列化 AffectedSystems
	var systemsJSON datatypes.JSON
	if len(req.AffectedSystems) > 0 {
		systemsJSON = marshalTicketTags(req.AffectedSystems)
	}

	// 序列化 ChatContext
	var chatCtxJSON datatypes.JSON
	if req.ChatContext != "" {
		// TODO(service/ticket): 校验 ChatContext 是合法 JSON。
		// 直接写入 datatypes.JSON 字符串，非法 JSON 会在数据库层报错且错误信息不友好。
		chatCtxJSON = datatypes.JSON(req.ChatContext)
	}

	ticket := &model.Ticket{
		TicketNo:        ticketNo,
		UserID:          userID,
		Title:           req.Title,
		Description:     req.Description,
		Urgency:         int16(req.Urgency),
		ImpactScope:     int16(req.ImpactScope),
		AffectedSystems: systemsJSON,
		ContactPhone:    req.ContactPhone,
		ContactEmail:    req.ContactEmail,
		ChatContext:     chatCtxJSON,
		Status:          1,
		Source:          1,
	}

	return s.repo.Create(ticket)
}

// =============================================================================
// SupplementTicket
// =============================================================================

// SupplementTicket 补充申告信息。
//
// 业务规则：
//   - 仅申告人本人可补充
//   - 仅"需补充信息"(3)状态可补充
//   - 补充后状态变为"处理中"(2)
//   - CreateRecord + UpdateStatus 在同一事务中原子执行
func (s *TicketService) SupplementTicket(id int64, userID int64, req request.SupplementTicketRequest) error {
	ticket, err := s.repo.FindByID(id)
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
	if ticket.Status != 3 {
		return AppError{Code: errcode.ErrParam, Message: "仅需补充信息状态可补充"}
	}

	// 事务内原子执行：CreateRecord + UpdateStatus，避免孤立记录
	return s.txManager.Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewTicketRepo(tx)

		record := &model.TicketRecord{
			TicketID:   id,
			OperatorID: userID,
			Action:     "supplement",
			Content:    req.Content,
		}
		if err := txRepo.CreateRecord(record); err != nil {
			return err
		}

		rows, err := txRepo.UpdateStatus(id, 2)
		if err != nil {
			return err
		}
		if rows == 0 {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}
		return nil
	})
}

// =============================================================================
// UpdateStatus
// =============================================================================

// UpdateStatus 执行申告状态转换。
//
// 状态机规则：
//
//	start:        待处理(1) → 处理中(2)
//	request_info: 处理中(2) → 需补充信息(3)，supplement_count+1，超过3次禁止
//	resolve:      处理中(2) → 已解决(4)
//	close:        任意状态 → 已关闭(5)
//
// 每次状态转换都会创建 TicketRecord，记录操作人、操作类型和结果描述。
//
// 为什么用 switch-case 而非状态转换矩阵：
// MVP 阶段 action 数量有限（4 个），switch-case 更直观且易于调试。
func (s *TicketService) UpdateStatus(id int64, operatorID int64, req request.UpdateTicketStatusRequest) error {
	// TODO(service/ticket): 状态机应使用 model.TicketAction* 和 model.TicketStatus* 常量。
	// 当前字符串和数字混用，和 enums.go 的常量定义没有形成编译期约束。
	ticket, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}
		return err
	}

	var newStatus int16
	var recordAction string

	switch req.Action {
	case "start":
		// 仅待处理(1)可开始处理
		if ticket.Status != 1 {
			return AppError{Code: errcode.ErrParam, Message: "仅待处理状态可开始处理"}
		}
		newStatus = 2
		recordAction = "start"

	case "request_info":
		// 仅处理中(2)可请求补充信息
		if ticket.Status != 2 {
			return AppError{Code: errcode.ErrParam, Message: "仅处理中状态可请求补充信息"}
		}
		// 原子自增 supplement_count，WHERE supplement_count < 3 保证并发安全
		ok, err := s.repo.IncrementSupplementCount(id)
		if err != nil {
			return err
		}
		if !ok {
			return AppError{Code: errcode.ErrParam, Message: "补充信息次数已达上限（3次）"}
		}
		newStatus = 3
		recordAction = "request_info"
		// TODO(service/ticket): request_info 后应同步创建站内消息或调用 MessageService.NotifySupplement。
		// 当前注释/文档说会通知，但 TicketService 构造函数未注入 MessageService，实际用户可能收不到补充提醒。

	case "resolve":
		// 仅处理中(2)可解决
		if ticket.Status != 2 {
			return AppError{Code: errcode.ErrParam, Message: "仅处理中状态可解决"}
		}
		newStatus = 4
		recordAction = "resolve"

	case "close":
		// 任意状态可关闭
		// TODO(service/ticket): close 是否允许关闭已解决/已关闭需要业务确认。
		// 当前重复关闭会再次创建 close 记录，可能污染处理时间线。
		newStatus = 5
		recordAction = "close"

	default:
		return AppError{Code: errcode.ErrParam, Message: "不支持的操作类型: " + req.Action}
	}

	// 包裹在事务中：UpdateStatus + CreateRecord 原子执行，
	// 避免状态已变但无 timeline 记录的数据不一致。
	err = s.txManager.Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewTicketRepo(tx)

		rows, err := txRepo.UpdateStatus(id, int(newStatus))
		if err != nil {
			return err
		}
		if rows == 0 {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}

		record := &model.TicketRecord{
			TicketID:   id,
			OperatorID: operatorID,
			Action:     recordAction,
			Content:    req.Result,
		}
		return txRepo.CreateRecord(record)
	})
	if err != nil {
		return err
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
func (s *TicketService) AddRecord(id int64, operatorID int64, req request.CreateTicketRecordRequest) error {
	// TODO(service/ticket): req.Detail 应校验为合法 JSON，并限制 action 白名单。
	// 处理记录是审计性质数据，非法 detail 或任意 action 会降低后续统计可信度。
	// 验证申告存在
	_, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}
		return err
	}

	var detailJSON datatypes.JSON
	if req.Detail != "" {
		detailJSON = datatypes.JSON(req.Detail)
	}

	record := &model.TicketRecord{
		TicketID:   id,
		OperatorID: operatorID,
		Action:     req.Action,
		Content:    req.Content,
		Detail:     detailJSON,
	}
	return s.repo.CreateRecord(record)
}

// =============================================================================
// ListByUser / ListAll / GetDetail
// =============================================================================

// ListByUser 分页查询当前用户的申告列表。
func (s *TicketService) ListByUser(userID int64, page, pageSize int) (*response.TicketListResponse, error) {
	tickets, total, err := s.repo.ListByUser(userID, page, pageSize)
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
func (s *TicketService) ListAll(status, urgency, page, pageSize int) (*response.TicketListResponse, error) {
	tickets, total, err := s.repo.ListAll(status, urgency, page, pageSize)
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
func (s *TicketService) GetDetail(id int64) (*response.TicketDetailResponse, error) {
	// TODO(service/ticket): 门户端查询详情时需要校验 ticket.UserID == currentUserID。
	// 当前 Handler 复用 GetDetail，Service 不知道调用者身份，存在水平越权风险。
	ticket, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, AppError{Code: errcode.ErrNotFound, Message: "申告不存在"}
		}
		return nil, err
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
	if len(ticket.AffectedSystems) > 0 {
		detail.AffectedSystems = unmarshalTicketTags(ticket.AffectedSystems)
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
		Urgency:         t.Urgency,
		ImpactScope:     t.ImpactScope,
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
func (s *TicketService) AutoClose(olderThan time.Time) (int64, error) {
	var closedCount int64

	err := s.txManager.Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewTicketRepo(tx)

		ids, err := txRepo.AutoCloseTickets(olderThan)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		now := time.Now()
		for _, id := range ids {
			record := &model.TicketRecord{
				TicketID:   id,
				OperatorID: 0, // 0 表示系统自动操作
				Action:     "auto_close",
				Content:    "系统自动关闭：申告超过 7 天未处理",
				CreatedAt:  now,
			}
			if err := tx.Create(record).Error; err != nil {
				return err
			}
		}

		closedCount = int64(len(ids))
		return nil
	})

	return closedCount, err
}
