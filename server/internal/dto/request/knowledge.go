// Package request 定义知识库管理相关的请求结构体。
package request

// CreateKBRequest 创建知识库请求。
type CreateKBRequest struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	EmbeddingModel  string `json:"embedding_model"`
	VectorDimension int    `json:"vector_dimension"`
	LlmConfigID     int64  `json:"llm_config_id"`
}

// UpdateKBRequest 更新知识库请求。
type UpdateKBRequest struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	EmbeddingModel  string `json:"embedding_model"`
	VectorDimension int    `json:"vector_dimension"`
}

// CreateArticleRequest 创建知识文章请求。
type CreateArticleRequest struct {
	KBID       int64    `json:"kb_id"`
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	SourceType int16    `json:"source_type"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
}

// UpdateArticleRequest 更新知识文章请求。
type UpdateArticleRequest struct {
	Title    string   `json:"title" binding:"required"`
	Content  string   `json:"content" binding:"required"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

// ReviewRequest 审核知识文章请求。
type ReviewRequest struct {
	Approved      bool   `json:"approved"`
	ReviewComment string `json:"review_comment"`
}
