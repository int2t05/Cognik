package database

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移所有数据模型。
//
// GORM AutoMigrate 会创建 GORM tag 中声明的索引，但不支持 DESC 排序。
// 以下索引需要 DESC 排序：
//   - idx_tickets_created_at ON tickets(created_at DESC)
//   - idx_chat_created_at ON chat_sessions(created_at DESC)
//   - idx_audit_created_at ON audit_logs(created_at DESC)
//
// 因此 AutoMigrate 完成后，通过原始 SQL 重建这三个索引为 DESC。
func AutoMigrate(db *gorm.DB) error {
	// TODO(database/migrate): AutoMigrate 不会启用 pgvector 扩展，也不会创建 halfvec/HNSW 专用索引。
	// 应将 pgvector schema 迁移固定到 SQL migration，确保开发、测试、生产结构完全一致。
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
		&model.LlmConfig{}, // 确保开发环境 AutoMigrate 也创建此表
		&model.ChatSession{},
		&model.ChatMessage{},
		&model.AuditLog{},
		&model.SystemConfig{},
		&model.Message{},
	); err != nil {
		return err
	}

	// 重建 DESC 索引（GORM AutoMigrate 只创建 ASC 索引）
	descIndexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_tickets_created_at ON tickets(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_chat_created_at ON chat_sessions(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_logs(created_at DESC)",
		// is_default 部分唯一索引：保证最多一个默认配置
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_default ON llm_configs(is_default) WHERE is_default = true",
	}
	// TODO(database/migrate): GORM AutoMigrate 已为 created_at 创建 ASC 索引，
	// 此处再创建 DESC 索引导致同一列存在两个索引，加倍存储和写入开销。
	// 应统一为单一索引策略（保留 DESC，通知 AutoMigrate 跳过 created_at 索引）。
	// 且这些索引 SQL 与 migrations/001_init.sql 之间存在重复定义风险，
	// 后续应选择单一迁移来源，避免 AutoMigrate 与 SQL migration 索引策略漂移。
	for _, sql := range descIndexes {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	return nil
}
