// Package database 负责初始化 PostgreSQL 数据库连接和 GORM 自动迁移。
package database

import (
	"fmt"

	"opsmind/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移所有数据模型和必要索引。
//
// 同时处理 GORM 无法覆盖的 pgvector 列：
// knowledge_chunks.embedding (halfvec) — VectorStore 通过原始 SQL 直接写入，Go model 无对应字段。
//
// 索引使用 IF NOT EXISTS：首次部署创建，后续启动跳过。
func AutoMigrate(db *gorm.DB) error {
	// 启用 pgvector 扩展（幂等——已存在不报错）
	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")

	if err := db.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.Menu{},
		&model.RoleMenu{},
		&model.Ticket{},
		&model.TicketRecord{},
		&model.KnowledgeBase{},
		&model.KnowledgeArticle{},
		&model.KnowledgeChunk{},
		&model.LlmConfig{},
		&model.ChatSession{},
		&model.ChatMessage{},
		&model.AuditLog{},
		&model.SystemConfig{},
		&model.Message{},
	); err != nil {
		return err
	}

	// 业务索引：首次部署创建，后续启动 IF NOT EXISTS 跳过
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_tickets_created_at ON tickets(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_chat_created_at ON chat_sessions(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_logs(created_at DESC)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_default ON llm_configs(is_default) WHERE is_default = true",
	}
	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	// 确保 knowledge_chunks.embedding (halfvec) 列存在。
	// GORM AutoMigrate 无法管理该列（Go model 无对应字段），VectorStore 通过原始 SQL 直接写入。
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

	return nil
}
