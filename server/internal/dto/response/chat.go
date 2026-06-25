// Package response 定义智能问答相关响应 DTO。
package response

// ChatSessionResponse 问答会话响应（含答案、来源和历史消息）。
type ChatSessionResponse struct {
	SessionID       int64          `json:"session_id"`
	KBID            int64          `json:"kb_id"`
	Question        string         `json:"question"`
	Answer          string         `json:"answer"`
	Sources         []SourceItem   `json:"sources"`
	Confidence      float64        `json:"confidence"`
	ConfidenceRaw   float64        `json:"confidence_raw"`
	ConfidenceLevel string         `json:"confidence_level"`
	CanSubmitTicket bool           `json:"can_submit_ticket"`
	DurationMS      int            `json:"duration_ms"`
	Feedback        int16          `json:"feedback"`
	CreatedAt       string         `json:"created_at"`
	Messages        []MessageItem  `json:"messages,omitempty"`   // 多轮对话历史（GetChatDetail 时返回）
	Pipeline        []PipelineStep `json:"pipeline,omitempty"`   // RAG 管道步骤指标
}

// PipelineStep RAG 管道步骤信息。
type PipelineStep struct {
	StepID     string `json:"step_id"`
	Label      string `json:"label"`
	DurationMS int64  `json:"duration_ms"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// MessageItem 对话消息条目（多轮对话历史）。
type MessageItem struct {
	ID              int64        `json:"id"`
	Role            string       `json:"role"` // "user" | "assistant"
	Content         string       `json:"content"`
	Sources         []SourceItem `json:"sources,omitempty"`
	Confidence      float64      `json:"confidence"`
	ConfidenceRaw   float64      `json:"confidence_raw"`
	ConfidenceLevel string       `json:"confidence_level"`
	Feedback        int16        `json:"feedback"` // 0=未反馈, 1=有帮助, 2=无帮助
	Status          string       `json:"status"`   // generating|completed|failed
	CreatedAt       string       `json:"created_at"`
}

// SessionListItem 会话列表条目（不含完整消息，仅摘要）。
type SessionListItem struct {
	ID           int64  `json:"id"`
	KBID         int64  `json:"kb_id"`        // 所属知识库 ID，前端编辑/更换 KB 时使用
	Question     string `json:"question"`      // 首轮问题，兼作会话标题
	LastAnswer   string `json:"last_answer"`   // 最后一条 assistant 回复（截断）
	MessageCount int64  `json:"message_count"` // 消息总数
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// SourceItem 知识来源条目。
type SourceItem struct {
	DocName      string  `json:"doc_name"`
	ChunkContent string  `json:"chunk_content"`
	Confidence   float64 `json:"confidence"`
}
