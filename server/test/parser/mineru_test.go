// Package parser_test 提供 parser 包及子包的集成测试。
//
// mineru_test.go 测试 MinerU 云端引擎的构造逻辑。
// 真实 API 调用仅在 MINERU_API_KEY 环境变量存在时执行，否则 t.Skip。
package parser_test

import (
	"bytes"
	"os"
	"testing"

	"cognos/internal/infra/config"
	"cognos/internal/parser/mineru"
)

// TestMinerU_NewEngine_NoKey 验证 API Key 为空时返回 nil（触发本地降级）。
func TestMinerU_NewEngine_NoKey(t *testing.T) {
	engine := mineru.NewEngine(config.MinerUConfig{
		APIKey:   "",
		Endpoint: "https://mineru.net/api/v4",
	})
	if engine != nil {
		t.Error("API Key 为空时 NewEngine 应返回 nil")
	}
}

// TestMinerU_NewEngine_WithKey 验证 API Key 非空时返回有效引擎实例。
func TestMinerU_NewEngine_WithKey(t *testing.T) {
	engine := mineru.NewEngine(config.MinerUConfig{
		APIKey:   "test-key",
		Endpoint: "https://mineru.net/api/v4",
	})
	if engine == nil {
		t.Fatal("API Key 非空时 NewEngine 应返回非 nil 引擎")
	}
}

// TestMinerU_NewEngine_DefaultEndpoint 验证 Endpoint 为空时使用默认值。
func TestMinerU_NewEngine_DefaultEndpoint(t *testing.T) {
	engine := mineru.NewEngine(config.MinerUConfig{
		APIKey: "test-key",
	})
	if engine == nil {
		t.Fatal("API Key 非空时 NewEngine 应返回非 nil 引擎")
	}
}

// TestMinerU_Parse_RealAPI 真实 MinerU API 调用测试。
//
// 仅在 MINERU_API_KEY 环境变量存在时执行，否则跳过。
// 测试用小型 TXT 文件（模拟 .md 扩展名）验证 MinerU 解析链路。
func TestMinerU_Parse_RealAPI(t *testing.T) {
	apiKey := os.Getenv("MINERU_API_KEY")
	if apiKey == "" {
		t.Skip("跳过 MinerU API 测试：未设置 MINERU_API_KEY 环境变量")
	}

	engine := mineru.NewEngine(config.MinerUConfig{
		APIKey:   apiKey,
		Endpoint: "https://mineru.net/api/v4",
	})
	if engine == nil {
		t.Fatal("API Key 存在时 NewEngine 应返回非 nil 引擎")
	}

	// 构造小型 PDF 测试文件
	pdfBytes := createMinimalPDF(t)

	result, err := engine.Parse(bytes.NewReader(pdfBytes), "test.pdf")
	if err != nil {
		t.Fatalf("MinerU 解析失败: %v", err)
	}
	if result == nil {
		t.Fatal("MinerU 解析结果不应为 nil")
	}
	if result.Markdown == "" {
		t.Error("MinerU 解析结果 Markdown 不应为空")
	}
}
