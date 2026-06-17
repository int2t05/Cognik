package model

// 用户状态
const (
	StatusActive   int16 = 1 // 正常
	StatusInactive int16 = 2 // 冻结
)

// 工单紧急程度
const (
	TicketUrgencyLow    int16 = 1 // 低
	TicketUrgencyMedium int16 = 2 // 中
	TicketUrgencyHigh   int16 = 3 // 高
)

// 工单影响范围
const (
	ImpactPersonal int16 = 1 // 个人
	ImpactDept     int16 = 2 // 部门
	ImpactCompany  int16 = 3 // 全公司
)

// 工单状态
const (
	TicketStatusPending        int16 = 1 // 待处理
	TicketStatusProcessing     int16 = 2 // 处理中
	TicketStatusNeedSupplement int16 = 3 // 需补充信息
	TicketStatusResolved       int16 = 4 // 已解决
	TicketStatusClosed         int16 = 5 // 已关闭
)

// 工单来源
const (
	TicketSourcePortal int16 = 1 // 门户提交
	TicketSourceChat   int16 = 2 // 问答转申告
)

// 工单操作类型
const (
	TicketActionStart       = "start"        // 开始处理
	TicketActionRequestInfo = "request_info" // 要求补充信息
	TicketActionSupplement  = "supplement"   // 补充信息
	TicketActionResolve     = "resolve"      // 解决
	TicketActionClose       = "close"        // 关闭
)

// 知识文章状态（审核状态机，承载人工操作流转）
//
// 编号说明：Disabled=0 是历史遗留值——早期 Service 直接用 `int16(0)` 作为"软删除/停用"
// 哨兵，后续被纳入枚举后未重新编号。已与 docs/API/knowledge.md 状态机表对齐（2026-06-17）。
//
// 文档处理进度（pending/parsing/chunking/embedding/indexing/completed/failed）由
// 独立的 ProcessStatus 字段承载，与本枚举互不污染。
const (
	ArticleStatusDisabled  int16 = 0 // 已停用（仅允许 Published → Disabled）
	ArticleStatusDraft     int16 = 1 // 草稿
	ArticleStatusReviewing int16 = 2 // 待审核
	ArticleStatusApproved  int16 = 3 // 审核通过
	ArticleStatusPublished int16 = 4 // 已发布
	ArticleStatusRejected  int16 = 5 // 驳回
)

// Embedding 模型类型
const (
	EmbeddingTypeAPI   int16 = 1 // API 接入
	EmbeddingTypeLocal int16 = 2 // 本地部署
)

// 对话角色
const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"
)

// 问答反馈状态
const (
	ChatFeedbackUnset      int16 = 0 // 未反馈
	ChatFeedbackResolved   int16 = 1 // 已解决
	ChatFeedbackUnresolved int16 = 2 // 未解决
)

// 站内消息类型
const (
	MessageTypeTicketSupplement = "ticket_supplement" // 申告补充信息
	MessageTypeSystem           = "system"            // 系统通知
)

// 菜单类型
const (
	MenuTypeMenu   = "menu"   // 菜单
	MenuTypeButton = "button" // 按钮
)

// TicketStatusText 返回工单状态的中文描述。
func TicketStatusText(status int16) string {
	switch status {
	case TicketStatusPending:
		return "待处理"
	case TicketStatusProcessing:
		return "处理中"
	case TicketStatusNeedSupplement:
		return "需补充信息"
	case TicketStatusResolved:
		return "已解决"
	case TicketStatusClosed:
		return "已关闭"
	default:
		return "未知"
	}
}

// ArticleStatusText 返回文章审核状态的中文描述。
func ArticleStatusText(status int16) string {
	switch status {
	case ArticleStatusDisabled:
		return "已停用"
	case ArticleStatusDraft:
		return "草稿"
	case ArticleStatusReviewing:
		return "待审核"
	case ArticleStatusApproved:
		return "已通过"
	case ArticleStatusPublished:
		return "已发布"
	case ArticleStatusRejected:
		return "已驳回"
	default:
		return "未知"
	}
}

// ArticleSourceTypeText 返回文章来源类型的中文描述。
func ArticleSourceTypeText(sourceType int16) string {
	switch sourceType {
	case 1:
		return "手动创建"
	case 2:
		return "文档上传"
	default:
		return "未知"
	}
}

// TicketUrgencyText 返回紧急程度的中文描述。
func TicketUrgencyText(urgency int16) string {
	switch urgency {
	case TicketUrgencyLow:
		return "低"
	case TicketUrgencyMedium:
		return "中"
	case TicketUrgencyHigh:
		return "高"
	default:
		return "未知"
	}
}

// TicketImpactText 返回影响范围的中文描述。
func TicketImpactText(impact int16) string {
	switch impact {
	case ImpactPersonal:
		return "个人"
	case ImpactDept:
		return "部门"
	case ImpactCompany:
		return "全公司"
	default:
		return "未知"
	}
}

// ProcessStatusText 返回文档处理状态的中文描述。
func ProcessStatusText(processStatus string) string {
	switch processStatus {
	case "pending":
		return "待处理"
	case "parsing":
		return "解析中"
	case "chunking":
		return "分块中"
	case "embedding":
		return "向量化中"
	case "indexing":
		return "索引中"
	case "completed":
		return "已完成"
	case "failed":
		return "失败"
	default:
		return processStatus
	}
}
