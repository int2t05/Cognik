package database

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移所有数据模型。
//
// GORM AutoMigrate 创建 basic indexes（均为 ASC），
// 但以下业务场景需要 DESC 排序索引（查询最新记录）：
//   - tickets.created_at
//   - chat_sessions.created_at
//   - audit_logs.created_at
//
// 策略：先 DROP 旧 ASC 索引，再 CREATE DESC 索引，
// 避免 CREATE INDEX IF NOT EXISTS 因同名已存在（ASC）而静默跳过。
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

	// 重建 DESC 索引：先删后建，确保索引方向正确
	type indexSQL struct {
		drop   string
		create string
	}
	indexes := []indexSQL{
		{"DROP INDEX IF EXISTS idx_tickets_created_at", "CREATE INDEX idx_tickets_created_at ON tickets(created_at DESC)"},
		{"DROP INDEX IF EXISTS idx_chat_created_at", "CREATE INDEX idx_chat_created_at ON chat_sessions(created_at DESC)"},
		{"DROP INDEX IF EXISTS idx_audit_created_at", "CREATE INDEX idx_audit_created_at ON audit_logs(created_at DESC)"},
		// is_default 部分唯一索引：保证最多一个默认 LLM 配置
		{"", "CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_default ON llm_configs(is_default) WHERE is_default = true"},
	}
	for _, idx := range indexes {
		if idx.drop != "" {
			if err := db.Exec(idx.drop).Error; err != nil {
				return err
			}
		}
		if err := db.Exec(idx.create).Error; err != nil {
			return err
		}
	}

	return nil
}
