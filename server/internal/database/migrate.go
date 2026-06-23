package database

import (
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移所有数据模型和必要索引。
//
// 索引使用 IF NOT EXISTS：首次部署创建，后续启动跳过。
// 旧版 ASC 索引已在历史部署中通过 DROP+CREATE 修复，无需再次处理。
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

	return nil
}
