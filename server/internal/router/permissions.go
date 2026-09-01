// Package router 负责注册 Gin 路由。
//
// 本文件定义权限常量别名，指向 user 领域包中的权威定义。
// 为什么不在这里定义：user.RoleService 的 validPermissions 是权限白名单的权威来源，
// router 通过别名引用，避免双维护风险。
package router

import "opsmind/internal/domain/user"

const (
	PermUserManage      = user.PermUserManage
	PermTicketRead      = user.PermTicketRead
	PermTicketWrite     = user.PermTicketWrite
	PermTicketManage    = user.PermTicketManage
	PermKnowledgeRead   = user.PermKnowledgeRead
	PermKnowledgeWrite  = user.PermKnowledgeWrite
	PermKnowledgeCreate = user.PermKnowledgeCreate
	PermKnowledgeManage = user.PermKnowledgeManage
	PermKnowledgeReview = user.PermKnowledgeReview
	PermAuditRead       = user.PermAuditRead
	PermDashboardRead   = user.PermDashboardRead
	PermSystemConfig    = user.PermSystemConfig
)
