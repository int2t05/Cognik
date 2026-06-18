
package model_test

import (
	"testing"
	"time"

	"opsmind/internal/model"

	"gorm.io/datatypes"
)

// TestKnowledgeBase_Fields 验证 KnowledgeBase 模型字段定义
func TestKnowledgeBase_Fields(t *testing.T) {
	now := time.Now()
	kb := model.KnowledgeBase{
		Name:             "运维知识库",
		Description:      "常见运维问题解答",
		RAGWorkspaceSlug: "ops-kb",
		EmbeddingModel:   "text-embedding-3-small",
		VectorDimension:  1536,
		CreatedBy:        1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	kb.ID = 1

	if kb.Name != "运维知识库" {
		t.Errorf("Name = %q, 期望 运维知识库", kb.Name)
	}
	if kb.EmbeddingModel != "text-embedding-3-small" {
		t.Errorf("EmbeddingModel = %q, 期望 text-embedding-3-small", kb.EmbeddingModel)
	}
	if kb.VectorDimension != 1536 {
		t.Errorf("VectorDimension = %d, 期望 1536", kb.VectorDimension)
	}
	if kb.RAGWorkspaceSlug != "ops-kb" {
		t.Errorf("RAGWorkspaceSlug = %q, 期望 ops-kb", kb.RAGWorkspaceSlug)
	}
}

// TestKnowledgeArticle_Fields 验证 KnowledgeArticle 模型字段定义
func TestKnowledgeArticle_Fields(t *testing.T) {
	now := time.Now()
	tags := datatypes.JSON(`["OA","登录"]`)
	art := model.KnowledgeArticle{
		KBID:               1,
		Title:           "OA系统无法登录怎么办？",
		Content:             "请清除浏览器缓存后重试",
		Category:           "OA系统",
		Tags:               tags,
		Status:             model.ArticleStatusDraft,
		CreatedBy:          1,
		// RAGDocumentLocation 已移除
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	art.ID = 1

	if art.KBID != 1 {
		t.Errorf("KBID = %d, 期望 1", art.KBID)
	}
	if art.Status != model.ArticleStatusDraft {
		t.Errorf("Status = %d, 期望 %d", art.Status, model.ArticleStatusDraft)
	}
	if art.Tags == nil {
		t.Error("Tags 为 nil, 期望有值")
	}
}

// TestKnowledgeChunk_Fields 验证 KnowledgeChunk 模型字段。
func TestKnowledgeChunk_Fields(t *testing.T) {
	now := time.Now()
	chunk := model.KnowledgeChunk{
		ArticleID:       1,
		KBID:            1,
		Content:         "OA系统登录问题",
		ChunkIndex:      2,
		EmbeddingModel:  "text-embedding-3-small",
		VectorDimension: 3,
		CreatedAt:       now,
	}
	chunk.ID = 1

	if chunk.ArticleID != 1 {
		t.Errorf("ArticleID = %d, 期望 1", chunk.ArticleID)
	}
	if chunk.KBID != 1 {
		t.Errorf("KBID = %d, 期望 1", chunk.KBID)
	}
	if chunk.ChunkIndex != 2 {
		t.Errorf("ChunkIndex = %d, 期望 2", chunk.ChunkIndex)
	}
	if chunk.VectorDimension != 3 {
		t.Errorf("VectorDimension = %d, 期望 3", chunk.VectorDimension)
	}
	if chunk.EmbeddingModel != "text-embedding-3-small" {
		t.Errorf("EmbeddingModel = %q, 期望 text-embedding-3-small", chunk.EmbeddingModel)
	}
}
