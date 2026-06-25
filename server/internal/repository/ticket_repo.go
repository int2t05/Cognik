// Package repository 提供申告工单的数据访问层。
//
// TicketRepo 封装 tickets 和 ticket_records 表的 CRUD 操作，供 TicketService 调用。
// 为什么独立于 UserRepo：申告表涉及状态筛选、分页查询、批量关闭等复杂操作，
// 独立 Repo 更利于维护和测试。
package repository

import (
	"context"
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
func (r *TicketRepo) Create(ctx context.Context, ticket *model.Ticket) error {
	return r.db.WithContext(ctx).Create(ticket).Error
}

// FindByID 按 ID 查询申告，预加载 User 和 TicketRecords。
func (r *TicketRepo) FindByID(ctx context.Context, id int64) (*model.Ticket, error) {
	var ticket model.Ticket
	err := r.db.WithContext(ctx).Preload("User").
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

// BatchDelete 批量删除申告（含关联处理记录）。
func (r *TicketRepo) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// 先删关联记录，再删申告
	if err := r.db.WithContext(ctx).Where("ticket_id IN ?", ids).Delete(&model.TicketRecord{}).Error; err != nil {
		return 0, err
	}
	res := r.db.WithContext(ctx).Delete(&model.Ticket{}, ids)
	return res.RowsAffected, res.Error
}

// Update 更新申告全部字段。
func (r *TicketRepo) Update(ctx context.Context, ticket *model.Ticket) error {
	return r.db.WithContext(ctx).Save(ticket).Error
}

// UpdateStatus 以 CAS 方式更新申告状态。
func (r *TicketRepo) UpdateStatus(ctx context.Context, id int64, expectedStatus, newStatus int) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Where("id = ? AND status = ?", id, expectedStatus).
		Update("status", newStatus)
	return result.RowsAffected, result.Error
}

// IncrementSupplementCount 原子自增补充信息计数。
// WHERE supplement_count < 3 提供 SQL 级 CAS 并发安全，配合 Service 层前置检查形成纵深防御。
func (r *TicketRepo) IncrementSupplementCount(ctx context.Context, id int64) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.Ticket{}).Where("id = ? AND supplement_count < 3", id).
		UpdateColumn("supplement_count", gorm.Expr("supplement_count + 1"))
	return result.RowsAffected > 0, result.Error
}

// ListByUser 分页查询指定用户的申告列表。
func (r *TicketRepo) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]model.Ticket, int64, error) {
	var tickets []model.Ticket
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Ticket{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// ListAll 分页查询全部申告，支持按状态筛选。
func (r *TicketRepo) ListAll(ctx context.Context, status int, page, pageSize int) ([]model.Ticket, int64, error) {
	var tickets []model.Ticket
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Ticket{})
	if status >= 0 {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	// 批量查询用户名并填充
	if len(tickets) > 0 {
		ids := make([]int64, len(tickets))
		for i, t := range tickets {
			ids[i] = t.UserID
		}
		var users []model.User
		if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
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
func (r *TicketRepo) AutoCloseTickets(ctx context.Context, olderThan time.Time) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Raw(
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
func (r *TicketRepo) CreateRecord(ctx context.Context, record *model.TicketRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

// FindByTicketID 按申告 ID 查询全部处理记录。
func (r *TicketRepo) FindByTicketID(ctx context.Context, ticketID int64) ([]model.TicketRecord, error) {
	var records []model.TicketRecord
	err := r.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}
