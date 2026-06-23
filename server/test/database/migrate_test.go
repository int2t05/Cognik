//go:build integration

package database_test

import (
	"testing"

	"opsmind/internal/database"
	"opsmind/internal/model"
)

// TestAutoMigrate_AllTablesCreated 验证 AutoMigrate 创建所有 16 张表
func TestAutoMigrate_AllTablesCreated(t *testing.T) {
	db, err := database.Init(testDBConfig())
	if err != nil {
		t.Fatalf("Init() 失败: %v", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() 失败: %v", err)
	}

	expectedTables := []string{
		"users",
		"roles",
		"user_roles",
		"menus",
		"role_menus",
		"tickets",
		"ticket_records",
		"knowledge_bases",
		"knowledge_articles",
		"knowledge_chunks",
		"chat_sessions",
		"chat_messages",
		"audit_logs",
		"system_configs",
		"messages",
	}

	for _, table := range expectedTables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("表 %s 不存在", table)
		}
	}
}

// TestAutoMigrate_UsersColumns 验证 users 表关键列存在
func TestAutoMigrate_UsersColumns(t *testing.T) {
	db, err := database.Init(testDBConfig())
	if err != nil {
		t.Fatalf("Init() 失败: %v", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() 失败: %v", err)
	}

	expectedColumns := []string{"username", "password_hash", "real_name", "phone", "email", "status", "first_login"}
	for _, col := range expectedColumns {
		if !db.Migrator().HasColumn(&model.User{}, col) {
			t.Errorf("users 表缺少列 %s", col)
		}
	}
}

// TestAutoMigrate_TicketsColumns 验证 tickets 表关键列存在
func TestAutoMigrate_TicketsColumns(t *testing.T) {
	db, err := database.Init(testDBConfig())
	if err != nil {
		t.Fatalf("Init() 失败: %v", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() 失败: %v", err)
	}

	expectedColumns := []string{"ticket_no", "user_id", "title", "urgency", "status", "source", "supplement_count"}
	for _, col := range expectedColumns {
		if !db.Migrator().HasColumn(&model.Ticket{}, col) {
			t.Errorf("tickets 表缺少列 %s", col)
		}
	}
}

// TestAutoMigrate_KnowledgeChunksColumns 验证 knowledge_chunks 表关键列存在
func TestAutoMigrate_KnowledgeChunksColumns(t *testing.T) {
	db, err := database.Init(testDBConfig())
	if err != nil {
		t.Fatalf("Init() 失败: %v", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() 失败: %v", err)
	}

	expectedColumns := []string{"article_id", "kb_id", "content", "chunk_index", "embedding_model", "vector_dimension"}
	for _, col := range expectedColumns {
		if !db.Migrator().HasColumn(&model.KnowledgeChunk{}, col) {
			t.Errorf("knowledge_chunks 表缺少列 %s", col)
		}
	}
}
