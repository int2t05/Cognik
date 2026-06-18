// Package router 负责注册 Gin 路由。
//
// 本文件定义权限常量——路由注册、RBAC 中间件、角色种子数据共享同一来源。
package router

const (
	PermUserManage      = "user:manage"
	PermTicketRead      = "ticket:read"
	PermTicketWrite     = "ticket:write"
	PermTicketManage    = "ticket:manage"
	PermKnowledgeRead   = "knowledge:read"
	PermKnowledgeWrite  = "knowledge:write"
	PermKnowledgeCreate = "knowledge:create"
	PermKnowledgeManage = "knowledge:manage"
	PermKnowledgeReview = "knowledge:review"
	PermAuditRead       = "audit:read"
	PermDashboardRead   = "dashboard:read"
	PermSystemConfig    = "system:config"
)
