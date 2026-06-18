//go:build ignore

// setup 初始化测试数据库表结构。
//
// 运行方式：go run tests/setup/main.go
package main

import (
	"log"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
)

func main() {
	db, err := database.Init(config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// 启用 pgvector 扩展
	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")

	models := []interface{}{
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.Menu{},
		&model.RoleMenu{},
		&model.AuditLog{},
		&model.SystemConfig{},
		&model.Message{},
		&model.LlmConfig{},
		&model.KnowledgeBase{},
		&model.KnowledgeArticle{},
		&model.KnowledgeChunk{},
		&model.Ticket{},
		&model.TicketRecord{},
		&model.ChatSession{},
		&model.ChatMessage{},
	}

	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			log.Fatalf("AutoMigrate %T 失败: %v", m, err)
		}
		log.Printf("✓ %T", m)
	}

	log.Println("测试数据库初始化完成")
}
