// Package response 定义申告管理相关响应 DTO。
//
// 列表响应包含分页信息，详情响应包含提交人信息和处理记录时间线。
package response

// TicketListResponse 申告列表响应（分页）。
type TicketListResponse struct {
	Tickets []TicketItem `json:"items"`
	Total   int64        `json:"total"`
}

// TicketItem 列表中的申告条目（精简字段）。
type TicketItem struct {
	ID              int64    `json:"id"`
	TicketNo        string   `json:"ticket_no"`
	UserID          int64    `json:"user_id"`
	SubmitterName   string   `json:"submitter_name"`
	Title           string   `json:"title"`
	Tags            []string `json:"tags"`
	ContactPhone    string   `json:"contact_phone"`
	Status          int16    `json:"status"`
	StatusText      string   `json:"status_text"`
	SupplementCount int16    `json:"supplement_count"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// TicketDetailResponse 申告详情响应（含提交人信息和处理记录时间线）。
type TicketDetailResponse struct {
	TicketItem
	Description  string             `json:"description"`
	ContactEmail string             `json:"contact_email"`
	Source       int16              `json:"source"`
	Records      []TicketRecordItem `json:"records"`
}

// TicketRecordItem 处理记录条目。
type TicketRecordItem struct {
	ID         int64  `json:"id"`
	TicketID   int64  `json:"ticket_id"`
	OperatorID int64  `json:"operator_id"`
	Action     string `json:"action"`
	Content    string `json:"content"`
	Detail     string `json:"detail"`
	CreatedAt  string `json:"created_at"`
}
