// Package database 负责初始化 PostgreSQL 数据库连接、GORM 自动迁移和种子数据。
package database

import (
	"fmt"
	"os"

	"opsmind/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移所有数据模型和必要索引。
//
// 向量维度固定为 1024（pgvector halfvec 列 + HNSW 索引）。
// 更换 embedding 模型时必须使用同为 1024 维的模型（如 bge-m3、bge-large-zh-v1.5）。
func AutoMigrate(db *gorm.DB) error {
	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")

	if err := db.AutoMigrate(
		&model.User{}, &model.Role{}, &model.UserRole{}, &model.Menu{}, &model.RoleMenu{},
		&model.Ticket{}, &model.TicketRecord{},
		&model.KnowledgeBase{}, &model.KnowledgeArticle{}, &model.KnowledgeChunk{},
		&model.LlmConfig{}, &model.ChatSession{}, &model.ChatMessage{},
		&model.AuditLog{}, &model.SystemConfig{}, &model.Message{},
	); err != nil {
		return err
	}

	for _, sql := range []string{
		"CREATE INDEX IF NOT EXISTS idx_tickets_created_at ON tickets(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_chat_created_at ON chat_sessions(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_logs(created_at DESC)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_default ON llm_configs(is_default) WHERE is_default = true",
	} {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	// halfvec(1024) 列：固定维度，支持 HNSW 索引
	if err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'knowledge_chunks' AND column_name = 'embedding'
			) THEN
				ALTER TABLE knowledge_chunks ADD COLUMN embedding halfvec(1024);
			END IF;
		END $$;
	`).Error; err != nil {
		return fmt.Errorf("添加 knowledge_chunks.embedding 列失败: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON knowledge_chunks
			USING hnsw (embedding halfvec_cosine_ops)
			WITH (m = 16, ef_construction = 200)
	`).Error; err != nil {
		return fmt.Errorf("创建 HNSW 索引失败: %w", err)
	}

	return nil
}

// AutoSeed 加载种子数据（角色/用户/菜单/LLM 配置/系统配置）。
//
// 通过检查 roles 表是否有数据来判断是否已加载，避免重复执行。
// 种子 SQL 文件路径默认为 ./migrations/seed_essential.sql（Docker 镜像内 /app/migrations/）。
func AutoSeed(db *gorm.DB) error {
	var count int64
	if err := db.Table("roles").Count(&count).Error; err != nil {
		return fmt.Errorf("检查种子数据失败: %w", err)
	}
	if count > 0 {
		return nil
	}

	path := "./migrations/seed_essential.sql"
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取种子数据文件 %s: %w", path, err)
	}

	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		return fmt.Errorf("加载种子数据失败: %w", err)
	}

	return nil
}
