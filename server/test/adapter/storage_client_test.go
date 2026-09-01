// Package adapter_test 验证 StorageClient 适配器的 MinIO 实现。
//
// 测试覆盖目录式存储接口：UploadFile / GetFileURL / DeleteDir / DownloadDir。
// 使用本地 MinIO 实例（localhost:9000），不可用时跳过测试。
//
//
// - Bucket 规划：opsmind-attachments（申告附件）、opsmind-documents（知识文档）
// - 初始化时自动创建 bucket
package adapter_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"opsmind/internal/infra/storage"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// tryConnectMinIO 尝试连接本地 MinIO 并返回客户端，不可用时跳过测试。
func tryConnectMinIO(t *testing.T) *minio.Client {
	t.Helper()

	client, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Skipf("无法创建 MinIO 客户端: %v", err)
	}

	// 健康检查：列出 buckets
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ListBuckets(ctx); err != nil {
		t.Skipf("MinIO 不可用（跳过集成测试）: %v", err)
	}

	return client
}

// =============================================================================
// UploadFile 测试
// =============================================================================

func TestStorageClient_Upload(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := storage.NewMinIOClient(rawClient, "opsmind-test-attachments")

	content := strings.Repeat("测试文件内容\n", 100)
	reader := bytes.NewReader([]byte(content))

	ctx := context.Background()
	err = client.UploadFile(ctx, "opsmind-test-attachments", "test", "upload.txt", reader, int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证文件存在
	key := "test/upload.txt"
	_, err = rawClient.StatObject(ctx, "opsmind-test-attachments", key, minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("上传后文件应存在: %v", err)
	}

	// 清理
	rawClient.RemoveObject(ctx, "opsmind-test-attachments", key, minio.RemoveObjectOptions{})
}

func TestStorageClient_Upload_EmptyContent(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := storage.NewMinIOClient(rawClient, "opsmind-test-attachments")

	reader := bytes.NewReader([]byte{})
	ctx := context.Background()
	err = client.UploadFile(ctx, "opsmind-test-attachments", "test", "empty.txt", reader, 0, "text/plain")
	if err != nil {
		t.Fatalf("空文件上传不应报错: %v", err)
	}

	// 清理
	rawClient.RemoveObject(ctx, "opsmind-test-attachments", "test/empty.txt", minio.RemoveObjectOptions{})
}

// =============================================================================
// GetFileURL 测试
// =============================================================================

func TestStorageClient_GetFileURL(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := storage.NewMinIOClient(rawClient, "opsmind-test-presigned")

	// 先上传文件
	content := []byte("预签名测试")
	ctx := context.Background()
	err = client.UploadFile(ctx, "opsmind-test-presigned", "test", "presigned.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}

	// 获取文件 URL
	url, err := client.GetFileURL(ctx, "opsmind-test-presigned", "test", "presigned.txt")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if !strings.Contains(url, "presigned.txt") {
		t.Errorf("预签名 URL 应包含文件名, got '%s'", url)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Error("预签名 URL 应包含签名参数")
	}

	// 清理
	rawClient.RemoveObject(ctx, "opsmind-test-presigned", "test/presigned.txt", minio.RemoveObjectOptions{})
}

// =============================================================================
// DeleteDir 测试
// =============================================================================

func TestStorageClient_DeleteDir(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := storage.NewMinIOClient(rawClient, "opsmind-test-delete")

	// 先上传文件
	content := []byte("待删除文件")
	ctx := context.Background()
	err = client.UploadFile(ctx, "opsmind-test-delete", "test", "to-delete.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}

	// 删除目录
	err = client.DeleteDir(ctx, "opsmind-test-delete", "test")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证文件已删除
	_, err = rawClient.StatObject(ctx, "opsmind-test-delete", "test/to-delete.txt", minio.StatObjectOptions{})
	if err == nil {
		t.Error("删除后文件应不存在")
	}
}

func TestStorageClient_DeleteDir_NotFound(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := storage.NewMinIOClient(rawClient, "opsmind-test-delete")

	// 删除不存在的目录不应报错（幂等性）
	// DeleteDir 列出空目录时直接返回 nil
	err = client.DeleteDir(context.Background(), "opsmind-test-delete", "nonexistent")
	if err != nil {
		t.Fatalf("删除不存在的目录不应报错（幂等）, got %v", err)
	}
}

// =============================================================================
// 文件读写验证（端到端）
// =============================================================================

func TestStorageClient_UploadDownloadRoundTrip(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := storage.NewMinIOClient(rawClient, "opsmind-test-roundtrip")

	original := []byte("端到端测试数据: 你好，世界！")
	ctx := context.Background()

	// 上传
	err = client.UploadFile(ctx, "opsmind-test-roundtrip", "test", "roundtrip.txt", bytes.NewReader(original), int64(len(original)), "text/plain")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}

	// 获取文件 URL
	url, err := client.GetFileURL(ctx, "opsmind-test-roundtrip", "test", "roundtrip.txt")
	if err != nil {
		t.Fatalf("获取文件 URL 失败: %v", err)
	}
	if url == "" {
		t.Error("文件 URL 不应为空")
	}

	// 通过 DownloadDir 下载并验证内容
	files, err := client.DownloadDir(ctx, "opsmind-test-roundtrip", "test")
	if err != nil {
		t.Fatalf("下载目录失败: %v", err)
	}
	reader, ok := files["roundtrip.txt"]
	if !ok {
		t.Fatalf("下载结果应包含 roundtrip.txt, 实际 keys: %v", files)
	}
	defer reader.Close()

	downloaded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if string(downloaded) != string(original) {
		t.Errorf("内容不一致: 期望 '%s', got '%s'", string(original), string(downloaded))
	}

	// 关闭其余 reader
	for _, r := range files {
		if r != reader {
			r.Close()
		}
	}

	// 清理
	rawClient.RemoveObject(ctx, "opsmind-test-roundtrip", "test/roundtrip.txt", minio.RemoveObjectOptions{})
}
