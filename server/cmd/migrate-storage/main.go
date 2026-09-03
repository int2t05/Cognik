// Package main — 一次性存储布局迁移入口。
//
// 将知识库文档从旧布局（双桶 + 标题目录）迁移到新布局（单桶 + kb/{draft|published}/article）。
// 用法: make migrate-storage （或 cd server && go run ./cmd/migrate-storage）
// 迁移完成后可删除本目录及 knowledge/migrate.go。
package main

import (
	"fmt"
	"log/slog"
	"os"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"opsmind/internal/domain/knowledge"
	"opsmind/internal/infra/config"
	"opsmind/internal/infra/database"
	"opsmind/internal/infra/storage"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	db, err := database.Init(cfg.Database)
	if err != nil {
		slog.Error("数据库连接失败", "error", err)
		os.Exit(1)
	}

	store, err := initStorage(cfg)
	if err != nil {
		slog.Error("存储初始化失败", "error", err)
		os.Exit(1)
	}

	if err := knowledge.MigrateStorageLayout(db, store); err != nil {
		slog.Error("存储布局迁移失败", "error", err)
		os.Exit(1)
	}
	slog.Info("存储布局迁移成功")
}

// initStorage 按 driver 初始化存储客户端（单桶）。
func initStorage(cfg *config.AppConfig) (storage.StorageClient, error) {
	bucket := cfg.Storage.Buckets.Documents
	switch cfg.Storage.Driver {
	case "minio":
		mc, err := minio.New(cfg.Storage.MinIO.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.Storage.MinIO.AccessKey, cfg.Storage.MinIO.SecretKey, ""),
			Secure: cfg.Storage.MinIO.UseSSL,
		})
		if err != nil {
			return nil, fmt.Errorf("MinIO 客户端创建失败: %w", err)
		}
		return storage.NewMinIOClient(mc, bucket)
	default:
		return storage.NewLocalStorageClient(cfg.Storage.Local.BaseDir, bucket)
	}
}
