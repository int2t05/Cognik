package model

import (
	"time"

	"gorm.io/datatypes"
)

// KnowledgeBase 知识库表
type KnowledgeBase struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string    `gorm:"type:varchar(128);not null" json:"name"`
	Description      string    `gorm:"type:text" json:"description"`
	RAGWorkspaceSlug string    `gorm:"type:varchar(128);uniqueIndex;column:rag_workspace_slug" json:"rag_workspace_slug"`
	EmbeddingModel   string    `gorm:"type:varchar(128);not null;column:embedding_model" json:"embedding_model"`
	VectorDimension  int       `gorm:"not null;column:vector_dimension" json:"vector_dimension"`
	LlmConfigID      int64     `gorm:"not null;default:0;column:llm_config_id" json:"llm_config_id"`
	CreatedBy        int64     `gorm:"column:created_by" json:"created_by"`
	CreatedAt        time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt        time.Time `gorm:"not null" json:"updated_at"`
}

func (KnowledgeBase) TableName() string { return "knowledge_bases" }

// KnowledgeArticle 知识文章表（统一文章模型：title + content + source_type）。
type KnowledgeArticle struct {
	ID              int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	KBID            int64          `gorm:"not null;column:kb_id" json:"kb_id"`
	KnowledgeBase   KnowledgeBase  `gorm:"foreignKey:KBID;references:ID" json:"knowledge_base,omitempty"`
	Title           string         `gorm:"type:varchar(255);not null;column:title" json:"title"`
	Content         string         `gorm:"type:text;not null;column:content" json:"content"`
	Category        string         `gorm:"type:varchar(64)" json:"category"`
	Tags            datatypes.JSON `gorm:"type:jsonb" json:"tags"`
	Status          int16          `gorm:"not null;default:1;index:idx_articles_status" json:"status"`
	SourceType      int16          `gorm:"not null;default:1;column:source_type" json:"source_type"`
	WordCount       int            `gorm:"not null;default:0;column:word_count" json:"word_count"`
	ChunkCount      int            `gorm:"not null;default:0;column:chunk_count" json:"chunk_count"`
	FileType        string         `gorm:"type:varchar(16);column:file_type" json:"file_type"`
	MinioPath       string         `gorm:"type:varchar(512);column:minio_path" json:"minio_path"`
	ProcessStatus   string         `gorm:"type:varchar(16);not null;default:pending;column:process_status;index:idx_articles_process_status" json:"process_status"`
	ProcessError    string         `gorm:"type:text;column:process_error" json:"process_error"`
	ContentHash     string         `gorm:"type:varchar(64);column:content_hash" json:"content_hash"` // SHA256，增量更新比对
	CreatedBy       int64          `gorm:"column:created_by" json:"created_by"`
	ReviewedBy      *int64         `gorm:"column:reviewed_by" json:"reviewed_by"`
	PublishedBy     *int64         `gorm:"column:published_by" json:"published_by"`
	ReviewComment   string         `gorm:"type:text;column:review_comment" json:"review_comment"`
	CreatedAt       time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"not null" json:"updated_at"`
}

func (KnowledgeArticle) TableName() string { return "knowledge_articles" }

// KnowledgeChunk 知识切片表。
// 记录知识条目发布时的切片内容和 pgvector 向量。
// embedding 向量以 halfvec 类型存储在 pgvector column 中（由 VectorStore 适配器通过 SQL 管理，不走 GORM）。
type KnowledgeChunk struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ArticleID       int64     `gorm:"not null;column:article_id;index:idx_chunks_article_id" json:"article_id"`
	KBID            int64     `gorm:"not null;default:0;column:kb_id;index:idx_chunks_kb_id" json:"kb_id"`
	Content         string    `gorm:"type:text;not null" json:"content"`
	ChunkIndex      int       `gorm:"not null;default:0;column:chunk_index" json:"chunk_index"`
	EmbeddingModel  string    `gorm:"type:varchar(128);not null;column:embedding_model" json:"embedding_model"`
	VectorDimension int       `gorm:"not null;column:vector_dimension" json:"vector_dimension"`
	ChunkHash       string    `gorm:"type:varchar(64);column:chunk_hash" json:"chunk_hash"` // SHA256，与正文增量比对
	CreatedAt       time.Time `gorm:"not null" json:"created_at"`
	// embedding halfvec(1024) — 由 VectorStore 适配器通过 SQL 直接管理，不走 GORM
}

func (KnowledgeChunk) TableName() string { return "knowledge_chunks" }
