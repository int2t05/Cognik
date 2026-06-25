package model

// 用户状态
const (
	StatusActive   int16 = 1 // 正常
	StatusInactive int16 = 2 // 冻结
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

// 文章来源类型
const (
	SourceTypeManual int16 = 1 // 手动创建
	SourceTypeUpload int16 = 2 // 文档上传
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
// Disabled=0 为历史遗留值：早期 Service 用 int16(0) 作软删除哨兵，
// 纳入枚举后保留编号不变。
//
// 文档处理进度由独立的 ProcessStatus 字段承载，与本枚举互不污染。
const (
	ArticleStatusDisabled  int16 = 0 // 已停用（仅允许 Published → Disabled）
	ArticleStatusDraft     int16 = 1 // 草稿
	ArticleStatusReviewing int16 = 2 // 待审核
	ArticleStatusApproved  int16 = 3 // 审核通过
	ArticleStatusPublished int16 = 4 // 已发布
	ArticleStatusRejected  int16 = 5 // 驳回
)

// 站内消息类型
//
// 新增类型时需同步更新前端 web/src/app/portal/messages/page.tsx 的 TYPE_LABEL 映射。
const (
	MessageTypeTicketSupplement  = "ticket_supplement"   // 申告补充信息
	MessageTypeTicketResolved    = "ticket_resolved"     // 申告已解决
	MessageTypeTicketClosed      = "ticket_closed"       // 申告已关闭
	MessageTypeKnowledgeApproved = "knowledge_approved"  // 知识文章审核通过
	MessageTypeKnowledgeRejected = "knowledge_rejected"  // 知识文章审核驳回
	MessageTypeSystem            = "system"              // 系统通知
)

// MessageTypeText 返回消息类型的中文描述。
func MessageTypeText(msgType string) string {
	switch msgType {
	case MessageTypeTicketSupplement:
		return "补充信息"
	case MessageTypeTicketResolved:
		return "已解决"
	case MessageTypeTicketClosed:
		return "已关闭"
	case MessageTypeKnowledgeApproved:
		return "审核通过"
	case MessageTypeKnowledgeRejected:
		return "审核驳回"
	case MessageTypeSystem:
		return "系统通知"
	default:
		return msgType
	}
}

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
	case SourceTypeManual:
		return "手动创建"
	case SourceTypeUpload:
		return "文档上传"
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
