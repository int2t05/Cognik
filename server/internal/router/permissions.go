// Package router 负责注册 Gin 路由。
//
// 本文件定义权限常量别名，指向 service 包中的权威定义。
// 为什么不在这里定义：service.RoleService 的 validPermissions 是权限白名单的权威来源，
// router 通过别名引用，避免双维护风险。
package router

import "opsmind/internal/service"

const (
	PermUserManage      = service.PermUserManage
	PermTicketRead      = service.PermTicketRead
	PermTicketWrite     = service.PermTicketWrite
	PermTicketManage    = service.PermTicketManage
	PermKnowledgeRead   = service.PermKnowledgeRead
	PermKnowledgeWrite  = service.PermKnowledgeWrite
	PermKnowledgeCreate = service.PermKnowledgeCreate
	PermKnowledgeManage = service.PermKnowledgeManage
	PermKnowledgeReview = service.PermKnowledgeReview
	PermAuditRead       = service.PermAuditRead
	PermDashboardRead   = service.PermDashboardRead
	PermSystemConfig    = service.PermSystemConfig
)
