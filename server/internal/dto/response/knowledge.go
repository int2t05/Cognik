// Package response 定义知识库管理相关的响应结构体。
package response

import "time"

// KBResponse 知识库响应。
type KBResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	// TODO(dto/knowledge): 门户端响应不应返回 rag_workspace_slug/embedding_model/vector_dimension/created_by。
	// 建议拆分 AdminKBResponse 和 PortalKBResponse，避免字段过曝。
	RAGWorkspaceSlug string `json:"rag_workspace_slug"`
	EmbeddingModel string `json:"embedding_model"`
	VectorDimension int  `json:"vector_dimension"`
	CreatedBy      int64  `json:"created_by"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	// TODO(dto/knowledge): 缺少 llm_config_id 和 article_count 字段，与 API 文档不一致。
	// Service 层 ListKBs 需补充 DB join 查询或子查询填充这两个字段。
}

// KBListResponse 知识库列表响应。
type KBListResponse struct {
	Items []KBResponse `json:"items"`
}

// ArticleListResponse 文章列表响应（含分页）。
type ArticleListResponse struct {
	Articles []ArticleResponse `json:"articles"`
	Total    int64             `json:"total"`
}

// ArticleResponse 文章列表项响应。
type ArticleResponse struct {
	ID            int64     `json:"id"`
	KBID          int64     `json:"kb_id"`
	KBName        string    `json:"kb_name"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Category      string    `json:"category"`
	Tags          []string  `json:"tags"`
	Status        int16     `json:"status"`
	StatusText    string    `json:"status_text"`
	SourceType    int16     `json:"source_type"`
	FileType      string    `json:"file_type"`
	MinioPath     string    `json:"minio_path"`
	WordCount     int       `json:"word_count"`
	ChunkCount    int       `json:"chunk_count"`
	// TODO(dto/knowledge): 缺少 source_type_text/created_by_name/published_by_name 字段，
	// 与 API 文档不一致。需 Service 层 join users 表填充姓名映射。
	ProcessStatus string    `json:"process_status"`
	ProcessError  string    `json:"process_error"`
	CreatedBy     int64     `json:"created_by"`
	ReviewedBy    *int64    `json:"reviewed_by"`
	PublishedBy   *int64    `json:"published_by"`
	ReviewComment string    `json:"review_comment"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ArticleDetailResponse 文章详情响应（含切片）。
type ArticleDetailResponse struct {
	ArticleResponse
	Chunks []ChunkResponse `json:"chunks"`
}

// DocumentUploadItem 文档上传响应项，替换原先的 gin.H 临时结构。
type DocumentUploadItem struct {
	ArticleID     int64  `json:"article_id"`
	FileName      string `json:"file_name"`
	FileType      string `json:"file_type"`
	ProcessStatus string `json:"process_status"`
}

// DocumentStatusResponse 文档处理状态响应，替换原先的 gin.H 临时结构。
type DocumentStatusResponse struct {
	ArticleID     int64  `json:"article_id"`
	FileName      string `json:"file_name"`
	ProcessStatus string `json:"process_status"`
	ProcessError  string `json:"process_error"`
}

// ChunkResponse 知识切片响应。
type ChunkResponse struct {
	// TODO(dto/knowledge): ChunkResponse 已有 kb_id/chunk_index，但缺少 created_at。
	// 如前端需要展示处理结果时间线，需保持完整字段。
	ID              int64  `json:"id"`
	KBID            int64  `json:"kb_id"`
	Content         string `json:"content"`
	ChunkIndex      int    `json:"chunk_index"`
	EmbeddingModel  string `json:"embedding_model"`
	VectorDimension int    `json:"vector_dimension"`
}
