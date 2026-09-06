// Package response 定义知识库管理相关的响应结构体。
package response

import "time"

// KBResponse 知识库响应（后台管理用）。
type KBResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	EmbeddingModel  string `json:"embedding_model"`
	VectorDimension int    `json:"vector_dimension"`
	ArticleCount    int    `json:"article_count"`
	CreatedBy       int64  `json:"created_by"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
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
	ID              int64     `json:"id"`
	KBID            int64     `json:"kb_id"`
	KBName          string    `json:"kb_name"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	Tags            []string  `json:"tags"`
	Status          int16     `json:"status"`
	StatusText      string    `json:"status_text"`
	SourceType      int16     `json:"source_type"`
	SourceTypeText  string    `json:"source_type_text"`
	ArticleType     string    `json:"article_type"`
	FileType        string    `json:"file_type"`
	MinioPath       string    `json:"minio_path"`
	WordCount       int       `json:"word_count"`
	ChunkCount      int       `json:"chunk_count"`
	ProcessStatus   string    `json:"process_status"`
	ProcessError    string    `json:"process_error"`
	CreatedBy       int64     `json:"created_by"`
	CreatedByName   string    `json:"created_by_name"`
	ReviewedBy      *int64    `json:"reviewed_by"`
	PublishedBy     *int64    `json:"published_by"`
	PublishedByName string    `json:"published_by_name"`
	ReviewComment   string    `json:"review_comment"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ArticleDetailResponse 文章详情响应（含切片）。
type ArticleDetailResponse struct {
	ArticleResponse
	Chunks []ChunkResponse `json:"chunks"`
}

// DocumentUploadResponse 文档上传响应（多文件）。
type DocumentUploadResponse struct {
	Documents []DocumentUploadItem `json:"documents"`
}

// UploadConfigResponse 文档上传配置响应（前端据此前置校验文件类型与大小）。
type UploadConfigResponse struct {
	MaxUploadSizeKB int      `json:"max_upload_size_kb"` // 上传大小上限(KB)
	MaxUploadSize   int64    `json:"max_upload_size"`    // 上传大小上限(字节)，前端直接用
	AllowedTypes    []string `json:"allowed_types"`      // 允许的文件类型列表
	MaxFiles        int      `json:"max_files"`           // 单次上传文件数上限
}

// DocumentUploadItem 单个文档上传响应项（并发上传时每个文件独立返回成功/失败）。
type DocumentUploadItem struct {
	ArticleID     int64  `json:"article_id"`     // 失败时为 0
	FileName      string `json:"file_name"`
	FileSize      int64  `json:"file_size"`
	FileType      string `json:"file_type"`       // 失败时可能为空
	ProcessStatus string `json:"process_status"`  // 失败时为空
	Success       bool   `json:"success"`         // 该文件是否上传成功
	ErrorMsg      string `json:"error_msg"`       // 失败原因（成功时为空）
}

// DocumentStatusResponse 文档处理状态响应。
type DocumentStatusResponse struct {
	ArticleID     int64  `json:"article_id"`
	FileName      string `json:"file_name"`
	ProcessStatus string `json:"process_status"`
	ProcessError  string `json:"process_error"`
}

// ChunkResponse 知识切片响应。
type ChunkResponse struct {
	ID              int64     `json:"id"`
	KBID            int64     `json:"kb_id"`
	Content         string    `json:"content"`
	ChunkIndex      int       `json:"chunk_index"`
	EmbeddingModel  string    `json:"embedding_model"`
	VectorDimension int       `json:"vector_dimension"`
	CreatedAt       time.Time `json:"created_at"`
}
