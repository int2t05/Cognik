// Package service 实现审计日志业务逻辑。
//
// audit_service.go 提供审计日志查询服务。
// 查询使用 LEFT JOIN 一次获取操作人姓名，避免二次查询。
package service

import (
	"opsmind/internal/dto/response"
	"opsmind/internal/repository"
)

// AuditService 审计日志查询服务。
type AuditService struct {
	auditRepo *repository.AuditRepo
}

// NewAuditService 创建 AuditService 实例。
func NewAuditService(auditRepo *repository.AuditRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo}
}

// List 分页查询审计日志（含操作人姓名，operatorID=0 映射为"系统"）。
func (s *AuditService) List(f repository.AuditFilter) ([]response.AuditLogItem, int64, error) {
	rows, total, err := s.auditRepo.List(f)
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
