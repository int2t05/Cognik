//go:build integration

// Package repository_test 验证 KnowledgeRepo 数据访问层。
//
// 测试覆盖 KnowledgeBase、KnowledgeArticle、KnowledgeChunk 的全部数据访问方法。
// EmbeddingConfig 已迁移至 llm_configs 表。
// 使用独立的 opsmind_test 数据库，每个测试用例通过清理保证隔离性。
package repository_test

import (
	"context"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// setupKnowledgeTestDB 初始化测试数据库连接并确保知识库相关表存在。
func setupKnowledgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbCfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind_test",
		SSLMode:  "disable",
	}

	db, err := database.Init(dbCfg)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_bases (
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(128) NOT NULL,
		description TEXT,
		rag_workspace_slug VARCHAR(128) UNIQUE,
		embedding_model VARCHAR(128) NOT NULL DEFAULT '',
		vector_dimension INTEGER NOT NULL DEFAULT 0,
		created_by BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 重建表以确保使用最新 schema
	db.Exec(`DROP TABLE IF EXISTS knowledge_articles CASCADE`)
	db.Exec(`CREATE TABLE knowledge_articles (
		id BIGSERIAL PRIMARY KEY,
		kb_id BIGINT NOT NULL,
		title VARCHAR(255) NOT NULL,
		content TEXT NOT NULL,
		category VARCHAR(64),
		tags JSONB,
		status SMALLINT NOT NULL DEFAULT 1,
		source_type SMALLINT NOT NULL DEFAULT 1,
		word_count INTEGER NOT NULL DEFAULT 0,
		chunk_count INTEGER NOT NULL DEFAULT 0,
		file_type VARCHAR(16) DEFAULT '',
		minio_path VARCHAR(512) DEFAULT '',
		process_status VARCHAR(16) NOT NULL DEFAULT 'completed',
		process_error TEXT DEFAULT '',
		created_by BIGINT NOT NULL DEFAULT 0,
		reviewed_by BIGINT,
		published_by BIGINT,
		review_comment TEXT DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_chunks (
		id BIGSERIAL PRIMARY KEY,
		article_id BIGINT NOT NULL,
		content TEXT NOT NULL,
		embedding_model VARCHAR(128) NOT NULL DEFAULT '',
		vector_dimension INTEGER NOT NULL DEFAULT 0,
		sync_status VARCHAR(16) NOT NULL DEFAULT 'pending',
		sync_error TEXT,
		synced_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	return db
}

// cleanKnowledgeTables 清理测试数据
func cleanKnowledgeTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	// 按 FK 依赖逆序清理，避免外键约束冲突
	db.Exec("DELETE FROM knowledge_chunks")
	db.Exec("DELETE FROM knowledge_articles")
	db.Exec("DELETE FROM knowledge_bases")
	db.Exec("DELETE FROM chat_messages")          // FK → chat_sessions
	db.Exec("DELETE FROM chat_sessions")          // FK → users/knowledge_bases
	db.Exec("DELETE FROM ticket_records")         // FK → tickets
	db.Exec("DELETE FROM tickets")                // FK → users
	db.Exec("DELETE FROM users WHERE username LIKE 'test_%'")
}

func TestKnowledgeRepo_CreateKB(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "测试知识库",
		Description:      "用于测试的知识库",
		RAGWorkspaceSlug: "test-workspace-slug",
		EmbeddingModel:   "text-embedding-3-small",
		VectorDimension:  1536,
		CreatedBy:        1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	err := repo.CreateKB(context.Background(), kb)
	require.NoError(t, err)
	assert.NotZero(t, kb.ID, "创建后应自动填充 ID")

	// 验证可查回
	got, err := repo.FindKBByID(context.Background(), kb.ID)
	require.NoError(t, err)
	assert.Equal(t, "测试知识库", got.Name)
	assert.Equal(t, "用于测试的知识库", got.Description)
	assert.Equal(t, "test-workspace-slug", got.RAGWorkspaceSlug)
	assert.Equal(t, "text-embedding-3-small", got.EmbeddingModel)
	assert.Equal(t, 1536, got.VectorDimension)
}

// TestKnowledgeRepo_FindKBByID_NotFound 查询不存在的知识库
func TestKnowledgeRepo_FindKBByID_NotFound(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := repository.NewKnowledgeRepo(db)

	got, err := repo.FindKBByID(context.Background(), 999999)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// TestKnowledgeRepo_UpdateKB 更新知识库
func TestKnowledgeRepo_UpdateKB(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "旧名称",
		Description:      "旧描述",
		RAGWorkspaceSlug: "old-slug",
		EmbeddingModel:   "old-model",
		VectorDimension:  768,
		CreatedBy:        1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb).Error)

	kb.Name = "新名称"
	kb.Description = "新描述"
	kb.UpdatedAt = time.Now()

	err := repo.UpdateKB(context.Background(), kb)
	require.NoError(t, err)

	got, err := repo.FindKBByID(context.Background(), kb.ID)
	require.NoError(t, err)
	assert.Equal(t, "新名称", got.Name)
	assert.Equal(t, "新描述", got.Description)
}

// TestKnowledgeRepo_ListKBs 列出全部知识库
func TestKnowledgeRepo_ListKBs(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb1 := &model.KnowledgeBase{
		Name:             "知识库1",
		RAGWorkspaceSlug: "slug-1",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	kb2 := &model.KnowledgeBase{
		Name:             "知识库2",
		RAGWorkspaceSlug: "slug-2",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb1).Error)
	require.NoError(t, db.Create(kb2).Error)

	kbs, err := repo.ListKBs(context.Background(), )
	require.NoError(t, err)
	assert.Len(t, kbs, 2)
	assert.True(t, kbs[0].Name == "知识库1" || kbs[0].Name == "知识库2")
}

// TestKnowledgeRepo_ListKBs_Empty 无知识库时返回空切片
func TestKnowledgeRepo_ListKBs_Empty(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	kbs, err := repo.ListKBs(context.Background(), )
	require.NoError(t, err)
	assert.Empty(t, kbs)
}

// =============================================================================
// KnowledgeArticle 测试
// =============================================================================

// TestKnowledgeRepo_CreateArticle 创建知识文章
func TestKnowledgeRepo_CreateArticle(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "文章测试库",
		RAGWorkspaceSlug: "article-test-slug",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb).Error)

	article := &model.KnowledgeArticle{
		KBID:      kb.ID,
		Title:  "如何重置密码？",
		Content:    "请访问设置页面，点击修改密码。",
		Category:  "账号管理",
		Tags:      datatypes.JSON(`["运维", "网络"]`),
		Status:    1,
		CreatedBy: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.CreateArticle(context.Background(), article)
	require.NoError(t, err)
	assert.NotZero(t, article.ID)

	// 验证可查回（含预加载 KnowledgeBase）
	got, err := repo.FindArticleByID(context.Background(), article.ID)
	require.NoError(t, err)
	assert.Equal(t, "如何重置密码？", got.Title)
	assert.Equal(t, "请访问设置页面，点击修改密码。", got.Content)
	assert.Equal(t, "账号管理", got.Category)
	assert.Equal(t, int16(1), got.Status)
	assert.Equal(t, kb.ID, got.KBID)
	// 验证预加载了 KnowledgeBase
	assert.NotNil(t, got.KnowledgeBase)
	assert.Equal(t, "文章测试库", got.KnowledgeBase.Name)
}

// TestKnowledgeRepo_FindArticleByID_NotFound 查询不存在的文章
func TestKnowledgeRepo_FindArticleByID_NotFound(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := repository.NewKnowledgeRepo(db)

	got, err := repo.FindArticleByID(context.Background(), 999999)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// TestKnowledgeRepo_UpdateArticle 更新文章
func TestKnowledgeRepo_UpdateArticle(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "更新测试库",
		RAGWorkspaceSlug: "update-test-slug",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb).Error)

	article := &model.KnowledgeArticle{
		KBID:      kb.ID,
		Title:  "旧问题",
		Content:    "旧答案",
		Status:    1,
		CreatedBy: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.Create(article).Error)

	article.Title = "新问题"
	article.Content = "新答案"
	article.UpdatedAt = time.Now()

	err := repo.UpdateArticle(context.Background(), article)
	require.NoError(t, err)

	got, err := repo.FindArticleByID(context.Background(), article.ID)
	require.NoError(t, err)
	assert.Equal(t, "新问题", got.Title)
	assert.Equal(t, "新答案", got.Content)
}

// TestKnowledgeRepo_ListArticles 分页查询文章列表（按知识库和状态过滤）
func TestKnowledgeRepo_ListArticles(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "列表测试库",
		RAGWorkspaceSlug: "list-test-slug",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb).Error)

	// 创建 3 篇文章：2 篇草稿，1 篇已发布
	for i := 0; i < 2; i++ {
		a := &model.KnowledgeArticle{
			KBID:      kb.ID,
			Title:  "问题" + string(rune('A'+i)),
			Content:    "答案" + string(rune('A'+i)),
			Status:    1, // 草稿
			CreatedBy: 1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, db.Create(a).Error)
	}

	published := &model.KnowledgeArticle{
		KBID:      kb.ID,
		Title:  "已发布问题",
		Content:    "已发布答案",
		Status:    3, // 已发布
		CreatedBy: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.Create(published).Error)

	// 查询草稿（status=1）
	articles, total, err := repo.ListArticles(context.Background(), kb.ID, 1, 0, "", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, articles, 2)

	// 查询已发布（status=3）
	articles, total, err = repo.ListArticles(context.Background(), kb.ID, 3, 0, "", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, articles, 1)
	assert.Equal(t, "已发布问题", articles[0].Title)

	// 查询全部（status=0）
	articles, total, err = repo.ListArticles(context.Background(), kb.ID, -1, 0, "", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, articles, 3)
}

// TestKnowledgeRepo_ListArticles_Pagination 分页功能
func TestKnowledgeRepo_ListArticles_Pagination(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "分页测试库",
		RAGWorkspaceSlug: "page-test-slug",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb).Error)

	for i := 0; i < 5; i++ {
		a := &model.KnowledgeArticle{
			KBID:      kb.ID,
			Title:  "问题",
			Content:    "答案",
			Status:    1,
			CreatedBy: 1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, db.Create(a).Error)
	}

	// 第 1 页，每页 2 条
	articles, total, err := repo.ListArticles(context.Background(), kb.ID, -1, 0, "", 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, articles, 2)

	// 第 2 页
	articles, total, err = repo.ListArticles(context.Background(), kb.ID, -1, 0, "", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, articles, 2)

	// 第 3 页（仅 1 条）
	articles, total, err = repo.ListArticles(context.Background(), kb.ID, -1, 0, "", 3, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, articles, 1)
}
// TestKnowledgeRepo_ListArticles_PreloadKnowledgeBase 验证 Preload KnowledgeBase 避免 N+1 查询。
//
// 修复前：ListArticles 缺少 .Preload("KnowledgeBase")，
// 每次访问 article.KnowledgeBase.Name 都会触发一次额外的 DB 查询（N+1 问题）。
// 修复后：KnowledgeBase 在列表查询时一并加载。
func TestKnowledgeRepo_ListArticles_PreloadKnowledgeBase(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "Preload测试库",
		RAGWorkspaceSlug: "preload-test-slug",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb).Error)

	article := &model.KnowledgeArticle{
		KBID:      kb.ID,
		Title:  "Preload测试",
		Content:    "验证Preload",
		Status:    1,
		CreatedBy: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.Create(article).Error)

	articles, _, err := repo.ListArticles(context.Background(), kb.ID, -1, 0, "", 1, 10)
	require.NoError(t, err)
	require.NotEmpty(t, articles)

	// KnowledgeBase 应被预加载，Name 不为空
	if articles[0].KnowledgeBase.Name == "" {
		t.Error("ListArticles 未预加载 KnowledgeBase — KBName 为空（N+1 查询风险）")
	}
	if articles[0].KnowledgeBase.Name != "Preload测试库" {
		t.Errorf("期望 KBName='Preload测试库', got '%s'", articles[0].KnowledgeBase.Name)
	}
}



// TestKnowledgeRepo_UpdateArticleStatus 更新文章状态
func TestKnowledgeRepo_UpdateArticleStatus(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	now := time.Now()
	kb := &model.KnowledgeBase{
		Name:             "状态测试库",
		RAGWorkspaceSlug: "status-test-slug",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Create(kb).Error)

	article := &model.KnowledgeArticle{
		KBID:      kb.ID,
		Title:  "状态问题",
		Content:    "状态答案",
		Status:    1, // 草稿
		CreatedBy: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.Create(article).Error)

	// 草稿 → 待审核
	err := repo.UpdateArticleStatus(context.Background(), article.ID, 2)
	require.NoError(t, err)

	got, err := repo.FindArticleByID(context.Background(), article.ID)
	require.NoError(t, err)
	assert.Equal(t, int16(2), got.Status)
}

// =============================================================================
// KnowledgeChunk 测试
// =============================================================================


// TestKnowledgeRepo_FindChunksByArticleID_Empty 无切片时返回空
func TestKnowledgeRepo_FindChunksByArticleID_Empty(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	cleanKnowledgeTables(t, db)
	repo := repository.NewKnowledgeRepo(db)

	got, err := repo.FindChunksByArticleID(context.Background(), 999999)
	require.NoError(t, err)
	assert.Empty(t, got)
}
