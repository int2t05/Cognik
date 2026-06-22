// Package database 提供数据库初始化和种子数据管理。
//
// AutoSeed 在首次启动时自动填充演示数据，后续启动检测到已有数据则跳过。
// 这样做的原因：开发环境 clone 后无需手动执行 make seed，开箱即用。
package database

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// AutoSeed 在数据库无种子数据时自动填充 001_init.sql 中的演示数据。
//
// 检测策略：统计 roles 表行数，>0 则跳过。
// 通过底层 *sql.DB 执行批量 SQL，每个语句块独立提交。
// 失败不阻塞启动——seed 数据缺失属于非致命错误。
func AutoSeed(db *gorm.DB) {
	var count int64
	if err := db.Raw("SELECT count(*) FROM roles").Scan(&count).Error; err != nil {
		slog.Warn("种子数据检测失败，跳过自动填充", "error", err)
		return
	}
	if count > 0 {
		slog.Info("数据库已有数据，跳过种子填充", "role_count", count)
		return
	}

	sqlPath := findSeedSQL()
	if sqlPath == "" {
		slog.Warn("种子 SQL 文件未找到，跳过自动填充")
		return
	}

	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		slog.Warn("读取种子 SQL 失败", "error", err)
		return
	}

	slog.Info("数据库为空，开始自动填充种子数据...", "path", sqlPath)

	// 提取演示数据区块（从 "-- 演示数据" 标记开始）
	content := strings.ReplaceAll(string(sqlBytes), "\r\n", "\n")
	idx := strings.Index(content, "-- 演示数据")
	if idx < 0 {
		slog.Warn("种子 SQL 中未找到演示数据标记")
		return
	}

	// 获取底层 *sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		slog.Warn("获取底层 DB 连接失败", "error", err)
		return
	}

	// 按空行分割为语句块（每个块是完整的 INSERT 或 SELECT setval）
	blocks := strings.Split(content[idx:], "\n\n")
	executed := 0

	for _, block := range blocks {
		// 去除注释行后判断
		trimmed := strings.TrimSpace(block)
		if trimmed == "" {
			continue
		}

		// 提取非注释行
		lines := strings.Split(trimmed, "\n")
		var sqlLines []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "--") {
				sqlLines = append(sqlLines, line)
			}
		}
		sql := strings.Join(sqlLines, "\n")
		if sql == "" {
			continue
		}

		// 跳过 DML 管理语句（DELETE / BEGIN / COMMIT）
		firstWord := strings.ToUpper(strings.Fields(sql)[0])
		if firstWord == "BEGIN" || firstWord == "COMMIT" || firstWord == "DELETE" {
			continue
		}

		if _, err := sqlDB.Exec(sql); err != nil {
			slog.Warn("种子 SQL 执行失败",
				"sql_preview", sql[:min(80, len(sql))],
				"error", err,
			)
		} else {
			executed++
		}
	}

	// 验证结果
	var roleCount int64
	db.Raw("SELECT count(*) FROM roles").Scan(&roleCount)
	slog.Info("种子数据填充完成",
		"statements_executed", executed,
		"roles_seeded", roleCount,
	)
}

// findSeedSQL 查找 001_init.sql 文件。
func findSeedSQL() string {
	paths := []string{
		"migrations/001_init.sql",
		filepath.Join("..", "migrations", "001_init.sql"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
