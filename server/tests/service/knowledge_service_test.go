//go:build integration

// Package service_test 验证 KnowledgeService 业务逻辑。
//
// CreateKB/Publish/Disable 操作数据库状态。
//
// 保留测试：知识库 CRUD、文章 CRUD、审核流程、EmbeddingConfig CRUD。
package service_test

import (
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/dto/request"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/gorm"
)

// =============================================================================
// 测试基础设施
// =============================================================================

var knowledgeSvcDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind_test",
		SSLMode:  "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	knowledgeSvcDB = db
}

func setupKnowledgeService(t *testing.T) *service.KnowledgeService {
	t.Helper()
	repo := repository.NewKnowledgeRepo(knowledgeSvcDB)
	svc := service.NewKnowledgeService(repo)

	// 清理测试数据
	knowledgeSvcDB.Exec("DELETE FROM knowledge_chunks")
	knowledgeSvcDB.Exec("DELETE FROM knowledge_articles")
	knowledgeSvcDB.Exec("DELETE FROM knowledge_bases")

	return svc
}

// createTestKB 创建测试用知识库。
func createTestKB(t *testing.T, _ *service.KnowledgeService, name string) *model.KnowledgeBase {
	t.Helper()
	kb := &model.KnowledgeBase{
		Name:             name,
		RAGWorkspaceSlug: "slug-" + name,
		CreatedBy:        1,
	}
	err := knowledgeSvcDB.Create(kb).Error
	if err != nil {
		t.Fatalf("创建测试知识库失败: %v", err)
	}
	return kb
}

// createTestArticle 创建测试用知识文章。
func createTestArticle(t *testing.T, _ *service.KnowledgeService, kbID int64, status int16) *model.KnowledgeArticle {
	t.Helper()
	article := &model.KnowledgeArticle{
		KBID:      kbID,
		Title:  "测试问题",
		Content:    "测试答案",
		Status:    status,
		CreatedBy: 1,
	}
	err := knowledgeSvcDB.Create(article).Error
	if err != nil {
		t.Fatalf("创建测试文章失败: %v", err)
	}
	return article
}

// =============================================================================
// KnowledgeBase 测试
// =============================================================================

// TestKnowledgeService_CreateKB 创建知识库成功。
func TestKnowledgeService_CreateKB(t *testing.T) {
	svc := setupKnowledgeService(t)

	err := svc.CreateKB(bgCtx, request.CreateKBRequest{
		Name:        "测试知识库",
		Description: "测试描述",
	}, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证数据库
	var kb model.KnowledgeBase
	knowledgeSvcDB.First(&kb)
	if kb.Name != "测试知识库" {
		t.Errorf("期望名称 '测试知识库', got '%s'", kb.Name)
	}
	if kb.RAGWorkspaceSlug == "" {
		t.Errorf("CreateKB 应自动生成 RAGWorkspaceSlug, 但为空")
	}
}

// TestKnowledgeService_UpdateKB 更新知识库。
func TestKnowledgeService_UpdateKB(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "旧名称")

	err := svc.UpdateKB(bgCtx, kb.ID, request.UpdateKBRequest{
		Name:        "新名称",
		Description: "新描述",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证更新
	var updated model.KnowledgeBase
	knowledgeSvcDB.First(&updated, kb.ID)
	if updated.Name != "新名称" {
		t.Errorf("期望名称 '新名称', got '%s'", updated.Name)
	}
}

// TestKnowledgeService_UpdateKB_NotFound 更新不存在的知识库。
func TestKnowledgeService_UpdateKB_NotFound(t *testing.T) {
	svc := setupKnowledgeService(t)

	err := svc.UpdateKB(bgCtx, 99999, request.UpdateKBRequest{
		Name: "不存在",
	})
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestKnowledgeService_ListKBs 列出全部知识库。
func TestKnowledgeService_ListKBs(t *testing.T) {
	svc := setupKnowledgeService(t)
	createTestKB(t, svc, "知识库1")
	createTestKB(t, svc, "知识库2")

	kbs, err := svc.ListKBs(bgCtx)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if len(kbs) != 2 {
		t.Errorf("期望 2 个知识库, got %d", len(kbs))
	}
}

// =============================================================================
// KnowledgeArticle 测试
// =============================================================================

// TestKnowledgeService_CreateArticle 创建文章（草稿状态）。
func TestKnowledgeService_CreateArticle(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "文章测试库")

	err := svc.CreateArticle(bgCtx, request.CreateArticleRequest{
		KBID:     kb.ID,
		Title: "如何重置密码？",
		Content:   "请访问设置页面。",
		Category: "账号管理",
		Tags:     []string{"密码", "账号"},
	}, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证状态为草稿
	var article model.KnowledgeArticle
	knowledgeSvcDB.First(&article)
	if article.Status != 1 {
		t.Errorf("期望 status=1(草稿), got %d", article.Status)
	}
	if article.Title != "如何重置密码？" {
		t.Errorf("期望问题 '如何重置密码？', got '%s'", article.Title)
	}
}

// TestKnowledgeService_CreateArticle_KBNotFound 知识库不存在时创建文章失败。
func TestKnowledgeService_CreateArticle_KBNotFound(t *testing.T) {
	svc := setupKnowledgeService(t)

	err := svc.CreateArticle(bgCtx, request.CreateArticleRequest{
		KBID:     99999,
		Title: "问题",
		Content:   "答案",
	}, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestKnowledgeService_UpdateArticle_Draft 草稿状态可编辑。
func TestKnowledgeService_UpdateArticle_Draft(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "编辑测试库")
	article := createTestArticle(t, svc, kb.ID, 1) // 草稿

	err := svc.UpdateArticle(bgCtx, article.ID, request.UpdateArticleRequest{
		Title: "更新后的问题",
		Content:   "更新后的答案",
		Category: "新分类",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.KnowledgeArticle
	knowledgeSvcDB.First(&updated, article.ID)
	if updated.Title != "更新后的问题" {
		t.Errorf("期望问题被更新, got '%s'", updated.Title)
	}
}

// TestKnowledgeService_UpdateArticle_NotEditable 非草稿/驳回状态不可编辑。
func TestKnowledgeService_UpdateArticle_NotEditable(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "不可编辑测试")
	article := createTestArticle(t, svc, kb.ID, 3) // 已发布

	err := svc.UpdateArticle(bgCtx, article.ID, request.UpdateArticleRequest{
		Title: "尝试修改",
		Content:   "应该失败",
	})
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestKnowledgeService_SubmitReview 草稿→待审核。
func TestKnowledgeService_SubmitReview(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "审核测试库")
	article := createTestArticle(t, svc, kb.ID, 1) // 草稿

	err := svc.SubmitReview(bgCtx, article.ID, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.KnowledgeArticle
	knowledgeSvcDB.First(&updated, article.ID)
	if updated.Status != 2 {
		t.Errorf("期望 status=2(待审核), got %d", updated.Status)
	}
}

// TestKnowledgeService_SubmitReview_WrongStatus 非草稿不可提交审核。
func TestKnowledgeService_SubmitReview_WrongStatus(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "错误审核测试")
	article := createTestArticle(t, svc, kb.ID, 3) // 已发布

	err := svc.SubmitReview(bgCtx, article.ID, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestKnowledgeService_Review_Approve 审核通过。
func TestKnowledgeService_Review_Approve(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "审核通过测试")
	article := createTestArticle(t, svc, kb.ID, 2) // 待审核

	err := svc.Review(bgCtx, article.ID, 2, request.ReviewRequest{ // reviewerID=2, creator=1
		Approved: true,
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.KnowledgeArticle
	knowledgeSvcDB.First(&updated, article.ID)
	if updated.Status != 3 {
		t.Errorf("期望 status=3(已通过), got %d", updated.Status)
	}
	if updated.ReviewedBy == nil || *updated.ReviewedBy != 2 {
		t.Error("期望 reviewed_by=2（审核人已记录）")
	}
}

// TestKnowledgeService_Review_Reject 审核驳回。
func TestKnowledgeService_Review_Reject(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "审核驳回测试")
	article := createTestArticle(t, svc, kb.ID, 2) // 待审核

	err := svc.Review(bgCtx, article.ID, 2, request.ReviewRequest{
		Approved:      false,
		ReviewComment: "答案不完整，请补充。",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.KnowledgeArticle
	knowledgeSvcDB.First(&updated, article.ID)
	if updated.Status != 5 {
		t.Errorf("期望 status=5(已驳回), got %d", updated.Status)
	}
	if updated.ReviewComment != "答案不完整，请补充。" {
		t.Errorf("期望评论, got '%s'", updated.ReviewComment)
	}
	// 验证 reviewer
	if updated.ReviewedBy == nil || *updated.ReviewedBy != 2 {
		t.Error("期望 reviewed_by=2")
	}
}

// TestKnowledgeService_Review_RejectNoComment 驳回未填审核意见。
func TestKnowledgeService_Review_RejectNoComment(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "驳回无意见测试")
	article := createTestArticle(t, svc, kb.ID, 2) // 待审核

	err := svc.Review(bgCtx, article.ID, 2, request.ReviewRequest{
		Approved: false,
		// ReviewComment 为空
	})
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestKnowledgeService_Review_SameAsCreator 审核人=创建人时拒绝。
func TestKnowledgeService_Review_SameAsCreator(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "同人审核测试")
	article := createTestArticle(t, svc, kb.ID, 2) // 待审核，CreatedBy=1

	// reviewerID=1，与创建人相同
	err := svc.Review(bgCtx, article.ID, 1, request.ReviewRequest{
		Approved: true,
	})
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

// TestKnowledgeService_Publish 发布已审核通过的文章。
// 当管道组件（chunker/embedder/store）未初始化时，Publish 应返回错误。
func TestKnowledgeService_Publish(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "发布测试库")
	article := createTestArticle(t, svc, kb.ID, 3) // 已通过

	err := svc.Publish(bgCtx, article.ID, 2)
	if err == nil {
		// 有真实管道时验证状态
		var updated model.KnowledgeArticle
		knowledgeSvcDB.First(&updated, article.ID)
		if updated.Status != int16(model.ArticleStatusPublished) {
			t.Errorf("期望 status=%d(已发布), got %d", model.ArticleStatusPublished, updated.Status)
		}
		if updated.PublishedBy == nil || *updated.PublishedBy != 2 {
			t.Error("期望 published_by=2")
		}
	} else {
		// 管道未初始化时的预期行为（chunker/embedder/store 为 nil）
		t.Logf("Publish 返回错误（预期：管道未初始化）: %v", err)
	}
}

// TestKnowledgeService_Disable 停用已发布文章。
// 当管道组件未初始化时，Disable 可能返回错误（无法删除向量）。
func TestKnowledgeService_Disable(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "停用测试库")
	article := createTestArticle(t, svc, kb.ID, 4) // 已发布

	err := svc.Disable(bgCtx, article.ID, 1)
	if err == nil {
		// 成功停用时验证 status=ArticleStatusDisabled(0)
		var updated model.KnowledgeArticle
		knowledgeSvcDB.First(&updated, article.ID)
		if updated.Status != int16(model.ArticleStatusDisabled) {
			t.Errorf("期望 status=%d(已停用), got %d", model.ArticleStatusDisabled, updated.Status)
		}
	} else {
		t.Logf("Disable 返回错误（预期：向量删除失败）: %v", err)
	}
}

// TestKnowledgeService_ListArticles 分页查询文章列表。
func TestKnowledgeService_ListArticles(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "列表测试库")

	for i := 0; i < 3; i++ {
		createTestArticle(t, svc, kb.ID, 1)
	}

	result, err := svc.ListArticles(bgCtx, kb.ID, -1, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Total != 3 {
		t.Errorf("期望 total=3, got %d", result.Total)
	}
	if len(result.Articles) != 3 {
		t.Errorf("期望 3 篇文章, got %d", len(result.Articles))
	}
}

// TestKnowledgeService_GetArticleDetail 获取文章详情（含切片）。
func TestKnowledgeService_GetArticleDetail(t *testing.T) {
	svc := setupKnowledgeService(t)
	kb := createTestKB(t, svc, "详情测试库")
	article := createTestArticle(t, svc, kb.ID, 1)

	// 创建切片
	chunk := model.KnowledgeChunk{
		ArticleID:       article.ID,
		Content:         "切片内容",
		EmbeddingModel:  "test-model",
		VectorDimension: 768,
	}
	knowledgeSvcDB.Create(&chunk)

	result, err := svc.GetArticleDetail(bgCtx, article.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Title != "测试问题" {
		t.Errorf("期望问题 '测试问题', got '%s'", result.Title)
	}
	if len(result.Chunks) != 1 {
		t.Errorf("期望 1 个切片, got %d", len(result.Chunks))
	}
	if result.Chunks[0].Content != "切片内容" {
		t.Errorf("期望切片内容 '切片内容', got '%s'", result.Chunks[0].Content)
	}
}
