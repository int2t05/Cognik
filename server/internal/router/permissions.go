// Package router 负责注册 Gin 路由。
//
// 本文件定义权限常量别名，指向 user 领域包中的权威定义。
// 为什么不在这里定义：user.RoleService 的 validPermissions 是权限白名单的权威来源，
// router 通过别名引用，避免双维护风险。
package router

import "opsmind/internal/domain/user/role"

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
