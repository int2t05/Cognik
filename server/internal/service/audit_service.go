// Package service 实现审计日志业务逻辑。
//
// audit_service.go 提供审计日志查询服务。
package service

import (
	"opsmind/internal/dto/response"
	"opsmind/internal/model"
	"opsmind/internal/repository"
)

// AuditService 审计日志查询服务。
type AuditService struct {
	auditRepo *repository.AuditRepo
	userRepo  *repository.UserRepo
}

// NewAuditService 创建 AuditService 实例。
func NewAuditService(auditRepo *repository.AuditRepo, userRepo *repository.UserRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo, userRepo: userRepo}
}

// List 分页查询审计日志，附加操作人姓名。
func (s *AuditService) List(f repository.AuditFilter) ([]response.AuditLogItem, int64, error) {
	logs, total, err := s.auditRepo.List(f)
	if err != nil {
		return nil, 0, err
	}

	operatorNames := s.batchGetOperatorNames(logs)

	items := make([]response.AuditLogItem, len(logs))
	for i, log := range logs {
		detail := ""
		if len(log.Detail) > 0 {
			detail = string(log.Detail)
		}
		name := operatorNames[log.OperatorID]
		if log.OperatorID == 0 {
			name = "系统"
		}
		items[i] = response.AuditLogItem{
			ID:           log.ID,
			OperatorID:   log.OperatorID,
			OperatorName: name,
			Action:       log.Action,
			TargetType:   log.TargetType,
			TargetID:     log.TargetID,
			Detail:       detail,
			IPAddress:    log.IPAddress,
			CreatedAt:    log.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return items, total, nil
}

func (s *AuditService) batchGetOperatorNames(logs []model.AuditLog) map[int64]string {
	if len(logs) == 0 {
		return make(map[int64]string)
	}
	idSet := make(map[int64]struct{}, len(logs))
	for _, log := range logs {
		idSet[log.OperatorID] = struct{}{}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	users, err := s.userRepo.FindByIDs(ids)
	if err != nil {
		return make(map[int64]string)
	}
	result := make(map[int64]string, len(users))
	for _, u := range users {
		result[u.ID] = u.RealName
	}
	return result
}
