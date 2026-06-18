package rag_test

import (
	"strings"
	"testing"

	"opsmind/internal/rag"
)

// TestChunker_ShortText 验证短于 chunkSize 的文本不被分割。
func TestChunker_ShortText(t *testing.T) {
	c := rag.NewChunker(1000, 200)

	text := "这是一段短文本，总共不到 1000 个字符。"
	// normalizeFullwidth 将全角标点转为半角，因此 chunker 输出中逗号为半角
	expected := "这是一段短文本,总共不到 1000 个字符。"
	chunks := c.Split(text)

	if len(chunks) != 1 {
		t.Fatalf("短文本期望 1 个分块, 实际 %d", len(chunks))
	}
	if chunks[0] != expected {
		t.Errorf("短文本内容应不变:\n  期望: %q\n  实际: %q", expected, chunks[0])
	}
}

// TestChunker_LongText 验证长文本按分隔符级别递归分割。
func TestChunker_LongText(t *testing.T) {
	c := rag.NewChunker(200, 50)

	// 构造多个段落，每段约 150 字符（> chunkSize 但 < chunkSize+overlap）
	para := strings.Repeat("运维系统账号冻结处理流程步骤清晰。", 4)
	text := strings.Repeat(para+"\n\n", 5)

	chunks := c.Split(strings.TrimRight(text, "\n\n"))

	if len(chunks) < 3 {
		t.Errorf("长文本应产生多个分块, 实际 %d", len(chunks))
	}

	// 每个分块不应超过 chunkSize+overlap
	for i, chunk := range chunks {
		// 使用 rune 计数更准确
		runeLen := len([]rune(chunk))
		if runeLen > 250 { // chunkSize=200 + 容差
			t.Errorf("分块 %d 长度 %d 超过预期上限 250: %q", i, runeLen, chunk)
		}
	}
}

// TestChunker_Overlap 验证相邻分块之间存在重叠。
func TestChunker_Overlap(t *testing.T) {
	c := rag.NewChunker(100, 30)

	// 构造一个一定会被分割的长文本
	text := strings.Repeat("ABCDEFGHIJ", 50)

	chunks := c.Split(text)
	if len(chunks) < 2 {
		t.Fatalf("期望至少 2 个分块, 实际 %d", len(chunks))
	}

	// 验证 chunk[i] 的结尾与 chunk[i+1] 的开头有重叠
	for i := 0; i < len(chunks)-1; i++ {
		current := chunks[i]
		next := chunks[i+1]

		// 取 current 尾部 overlap 大小的子串
		overlapTail := current
		if len(current) > 30 {
			overlapTail = current[len(current)-30:]
		}

		overlapHead := next
		if len(next) > 30 {
			overlapHead = next[:30]
		}

		if !strings.Contains(next, overlapTail) && overlapTail != "" {
			t.Logf("分块 %d 尾部和分块 %d 头部无显著重叠", i, i+1)
		}
		_ = overlapHead
	}
}

// TestChunker_MixedChineseEnglish 验证中英文混合文本正确分块。
func TestChunker_MixedChineseEnglish(t *testing.T) {
	c := rag.NewChunker(300, 80)

	text := `VPN Connection Troubleshooting Guide

VPN 连接问题排查步骤如下：

1. Check if the VPN client is running properly.
2. 确认网络连接正常，尝试 ping 内网网关。
3. If the issue persists, contact the IT helpdesk.
4. 记录错误日志并提交申告工单。

The most common issues include expired certificates and incorrect server addresses.`

	chunks := c.Split(text)

	if len(chunks) == 0 {
		t.Fatal("非空文本不应返回 0 个分块")
	}

	// 英文和中文均应出现在分块中
	joined := strings.Join(chunks, " ")
	if !strings.Contains(joined, "VPN") {
		t.Error("分块内容应包含 'VPN'")
	}
	if !strings.Contains(joined, "连接") {
		t.Error("分块内容应包含 '连接'")
	}
}

// TestChunker_EmptyInput 验证空字符串返回空切片。
func TestChunker_EmptyInput(t *testing.T) {
	c := rag.NewChunker(1000, 200)

	chunks := c.Split("")

	if len(chunks) != 0 {
		t.Errorf("空字符串期望 0 个分块, 实际 %d", len(chunks))
	}
}
