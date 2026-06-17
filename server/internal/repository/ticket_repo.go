// Package repository 提供申告工单的数据访问层。
//
// TicketRepo 封装 tickets 和 ticket_records 表的 CRUD 操作，供 TicketService 调用。
// 为什么独立于 UserRepo：申告表涉及状态筛选、分页查询、批量关闭等复杂操作，
// 独立 Repo 更利于维护和测试。
package repository

import (
	"time"

	"opsmind/internal/model"

	"gorm.io/gorm"
)

// TicketRepo 申告数据访问
type TicketRepo struct {
	db *gorm.DB
}

// NewTicketRepo 创建 TicketRepo 实例
func NewTicketRepo(db *gorm.DB) *TicketRepo {
	return &TicketRepo{db: db}
}

// =============================================================================
// Ticket
// =============================================================================

// Create 创建申告工单。
//
// 创建后 ticket.ID 会被 GORM 自动填充。
// ticket_no 唯一约束由数据库保证。
func (r *TicketRepo) Create(ticket *model.Ticket) error {
	return r.db.Create(ticket).Error
}

// FindByID 按 ID 查询申告，预加载 User 和 TicketRecords。
//
// 为什么预加载 User：详情页需要显示提交人信息（姓名、手机号）。
// 为什么预加载 TicketRecords：详情页需要展示处理记录时间线。
// TicketRecords 按 created_at 升序排列（最早在前）。
func (r *TicketRepo) FindByID(id int64) (*model.Ticket, error) {
	var ticket model.Ticket
	err := r.db.Preload("User").
		Preload("TicketRecords", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Where("id = ?", id).
		First(&ticket).Error
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

// Update 更新申告全部字段。
//
// 为什么用 Save 而非 Updates：Service 层修改多个字段后全量保存，
// Save 会更新所有字段（包括零值），确保数据一致性。
func (r *TicketRepo) Update(ticket *model.Ticket) error {
	return r.db.Save(ticket).Error
}

// UpdateStatus 以 CAS（Compare-And-Swap）方式更新申告状态。
//
// WHERE id=? AND status=? 保证只有当前状态匹配时才执行更新，
// 防止两操作者从同一旧状态并发操作产生双重记录。
// 返回 RowsAffected：1=成功，0=状态已变更或不存在。
func (r *TicketRepo) UpdateStatus(id int64, expectedStatus, newStatus int) (int64, error) {
	result := r.db.Model(&model.Ticket{}).
		Where("id = ? AND status = ?", id, expectedStatus).
		Update("status", newStatus)
	return result.RowsAffected, result.Error
}

// IncrementSupplementCount 原子自增补充信息计数。
//
// 使用 WHERE supplement_count < 3 条件保证原子性，并发请求不会被绕过上限。
// 返回 ok=true 表示自增成功，ok=false 表示已达上限（3 次）未执行自增。
func (r *TicketRepo) IncrementSupplementCount(id int64) (bool, error) {
	result := r.db.Model(&model.Ticket{}).Where("id = ? AND supplement_count < 3", id).
		UpdateColumn("supplement_count", gorm.Expr("supplement_count + 1"))
	return result.RowsAffected > 0, result.Error
}

// ListByUser 分页查询指定用户的申告列表。
//
// 按 id DESC 排序（最新在前），返回总数和列表。
func (r *TicketRepo) ListByUser(userID int64, page, pageSize int) ([]model.Ticket, int64, error) {
	var tickets []model.Ticket
	var total int64

	query := r.db.Model(&model.Ticket{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// ListAll 分页查询全部申告，支持按状态和紧急程度筛选。
//
// 参数说明：
//   - status: -1 表示不过滤，其他值按精确匹配
//   - urgency: 0 表示不过滤，其他值按精确匹配
func (r *TicketRepo) ListAll(status int, urgency int, page, pageSize int) ([]model.Ticket, int64, error) {
	// TODO(repository/ticket): ListAll 对提交人使用二次查询填充，但失败时静默忽略。
	// 如果用户表查询失败，应返回错误，避免前端看到空 submitterName 误判为匿名。
	var tickets []model.Ticket
	var total int64

	query := r.db.Model(&model.Ticket{})
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	if urgency > 0 {
		query = query.Where("urgency = ?", urgency)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	// 批量查询用户名并填充（GORM Preload 在 Count 后不可靠）
	if len(tickets) > 0 {
		ids := make([]int64, len(tickets))
		for i, t := range tickets {
			ids[i] = t.UserID
		}
		var users []model.User
		if err := r.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
			return nil, 0, err
		}
		userMap := make(map[int64]model.User, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}
		for i := range tickets {
			if u, ok := userMap[tickets[i].UserID]; ok {
				tickets[i].User = u
			}
		}
	}

	return tickets, total, nil
}

// AutoCloseTickets 原子关闭超期申告并返回被关闭的 ticket ID 列表。
//
// 使用 UPDATE ... RETURNING id 在单条 SQL 中完成"查询+关闭"，
// 消除 SELECT-then-UPDATE 的 TOCTOU 竞态窗口。
func (r *TicketRepo) AutoCloseTickets(olderThan time.Time) ([]int64, error) {
	var ids []int64
	err := r.db.Raw(
		`UPDATE tickets SET status = ?, updated_at = NOW()
		 WHERE status IN (?,?,?) AND created_at < ?
		 RETURNING id`,
		model.TicketStatusClosed,
		model.TicketStatusPending, model.TicketStatusProcessing, model.TicketStatusNeedSupplement,
		olderThan,
	).Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// =============================================================================
// TicketRecord
// =============================================================================

// CreateRecord 创建申告处理记录。
//
// 创建后 record.ID 会被 GORM 自动填充。
func (r *TicketRepo) CreateRecord(record *model.TicketRecord) error {
	return r.db.Create(record).Error
}

// FindByTicketID 按申告 ID 查询全部处理记录。
//
// 按 created_at ASC 排序（最早在前），形成处理时间线。
func (r *TicketRepo) FindByTicketID(ticketID int64) ([]model.TicketRecord, error) {
	var records []model.TicketRecord
	err := r.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}
