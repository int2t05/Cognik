// Package router 负责注册 Gin 路由。
package router

import "cognik/internal/domain/user/role"

const (
	PermUserManage      = role.PermUserManage
	PermTicketRead      = role.PermTicketRead
	PermTicketWrite     = role.PermTicketWrite
	PermTicketManage    = role.PermTicketManage
	PermKnowledgeRead   = role.PermKnowledgeRead
	PermKnowledgeWrite  = role.PermKnowledgeWrite
	PermKnowledgeCreate = role.PermKnowledgeCreate
	PermKnowledgeManage = role.PermKnowledgeManage
	PermKnowledgeReview = role.PermKnowledgeReview
	PermAuditRead       = role.PermAuditRead
	PermDashboardRead   = role.PermDashboardRead
	PermSystemConfig    = role.PermSystemConfig
)
