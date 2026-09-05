// Package audit 审计日志业务逻辑。
//
// AuditWriter 接口定义在此，各业务 Service 通过它写入审计日志。
package audit

import (
	"context"

	respDto "cognik/internal/shared/dto/response"
	"cognik/internal/shared/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuditWriter 审计日志写入接口。
type AuditWriter interface {
	// Write 写入审计日志（默认 DB 连接）。
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
	// WriteWithTx 在事务中写入审计日志。
	WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error
}

// AuditService 审计日志读写服务。
type AuditService struct {
	auditRepo *AuditRepo
}

// NewAuditService 创建 AuditService 实例。
func NewAuditService(auditRepo *AuditRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo}
}

// buildAuditLog 构造 AuditLog（detail 空写 NULL，非空写 JSON）。
func (s *AuditService) buildAuditLog(operatorID int64, action, targetType string, targetID int64, detail string) *model.AuditLog {
	var jsonDetail datatypes.JSON
	if detail != "" {
		jsonDetail = datatypes.JSON(detail)
	}
	return &model.AuditLog{
		OperatorID: operatorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     jsonDetail,
	}
}

// Write 写入审计日志（非事务）。
func (s *AuditService) Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error {
	return s.auditRepo.Create(ctx, s.buildAuditLog(operatorID, action, targetType, targetID, detail))
}

// WriteWithTx 在事务中写入审计日志。
func (s *AuditService) WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error {
	txRepo := NewAuditRepo(tx)
	return txRepo.Create(ctx, s.buildAuditLog(operatorID, action, targetType, targetID, detail))
}

// Create 直接写入审计日志记录。
func (s *AuditService) Create(ctx context.Context, log any) error {
	return s.auditRepo.Create(ctx, log)
}

// List 分页查询审计日志（含操作人姓名，operatorID=0 显示"系统"）。
func (s *AuditService) List(ctx context.Context, f AuditFilter) ([]respDto.AuditLogItem, int64, error) {
	rows, total, err := s.auditRepo.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	items := make([]respDto.AuditLogItem, len(rows))
	for i, row := range rows {
		name := row.OperatorName
		if row.OperatorID == 0 {
			name = "系统"
		}
		items[i] = respDto.AuditLogItem{
			ID:           row.ID,
			OperatorID:   row.OperatorID,
			OperatorName: name,
			Action:       row.Action,
			TargetType:   row.TargetType,
			TargetID:     row.TargetID,
			Detail:       row.Detail,
			IPAddress:    row.IPAddress,
			CreatedAt:    row.CreatedAt,
		}
	}

	return items, total, nil
}

// BatchDelete 批量删除审计日志。
func (s *AuditService) BatchDelete(ctx context.Context, ids []int64) (int64, error) {
	return s.auditRepo.BatchDelete(ctx, ids)
}
