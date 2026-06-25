// Package service 实现审计日志业务逻辑。
//
// AuditService 统一管理审计日志的读（查询/列表）和写（Write）。
// 审计日志写入由各 Service 通过 AuditWriter 接口触发，
// 不再直接调用 AuditRepo.Create——AuditService 是唯一的审计接缝。
package service

import (
	"context"
	"opsmind/internal/dto/response"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuditWriter 定义审计日志写入接口。
//
// 各 Service 通过此接口写入审计日志，而非直接依赖 AuditRepo。
// 这样做的好处：
//   - locality：审计格式（操作人、来源、IP）变更只需改 AuditService.Write，不影响 5 个调用方
//   - leverage：一个接口，5 个 Service 调用方
//   - testability：Service 测试可注入假 AuditWriter，无需构造完整 AuditRepo
type AuditWriter interface {
	// Write 写入一条审计日志（使用 AuditService 持有的默认 DB 连接）。
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
	// WriteWithTx 在事务中写入审计日志（用于 Service 层事务内审计）。
	// tx 为 GORM 事务句柄，审计写入与业务操作在同一事务中提交或回滚。
	WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error
}

// AuditFilter 审计日志查询过滤条件。
// 定义在 Service 层而非 Repository 层，避免 Handler 直接导入 Repository 类型。
type AuditFilter struct {
	OperatorID int64
	Action     string
	TargetType string
	TargetID   int64
	DateFrom   string
	DateTo     string
	Page       int
	PageSize   int
}

// AuditService 审计日志读写服务——唯一的审计接缝。
type AuditService struct {
	auditRepo *repository.AuditRepo
}

// NewAuditService 创建 AuditService 实例。
func NewAuditService(auditRepo *repository.AuditRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo}
}

// buildAuditLog 构造 model.AuditLog 并处理 detail 字段的类型转换。
// detail 为空字符串时写入 NULL（PostgreSQL jsonb 接受），
// detail 为非空字符串时预期为已编码的 JSON 字节序列。
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

// Write 实现 AuditWriter 接口——写入一条审计日志（非事务）。
// 调用方无需知道 model.AuditLog 的结构，只需传入业务字段。
func (s *AuditService) Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error {
	return s.auditRepo.Create(ctx, s.buildAuditLog(operatorID, action, targetType, targetID, detail))
}

// WriteWithTx 在事务中写入审计日志——用于 Service 层事务内审计。
// tx 为 GORM 事务句柄，审计写入与业务操作在同一事务中提交或回滚。
func (s *AuditService) WriteWithTx(ctx context.Context, tx *gorm.DB, operatorID int64, action, targetType string, targetID int64, detail string) error {
	txRepo := repository.NewAuditRepo(tx)
	return txRepo.Create(ctx, s.buildAuditLog(operatorID, action, targetType, targetID, detail))
}

// Create 直接写入审计日志记录（用于 ChatService 等已有完整 AuditLog 的调用方）。
// 实现 consumer-defined auditLogWriter 接口。
func (s *AuditService) Create(ctx context.Context, log any) error {
	return s.auditRepo.Create(ctx, log)
}

// List 分页查询审计日志（含操作人姓名，operatorID=0 映射为"系统"）。
func (s *AuditService) List(ctx context.Context, f AuditFilter) ([]response.AuditLogItem, int64, error) {
	repoFilter := repository.AuditFilter(f)
	rows, total, err := s.auditRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, 0, err
	}

	items := make([]response.AuditLogItem, len(rows))
	for i, row := range rows {
		name := row.OperatorName
		if row.OperatorID == 0 {
			name = "系统"
		}
		items[i] = response.AuditLogItem{
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
