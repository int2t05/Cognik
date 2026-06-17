// Package adapter_test 验证 StorageClient 适配器的 MinIO 实现。
//
// 测试覆盖 PLAN.md Task27 定义的 3 个方法：Upload / GetPresignedURL / Delete。
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

	"opsmind/internal/adapter"

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
// Upload 测试
// =============================================================================

func TestStorageClient_Upload(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := adapter.NewMinIOClient(rawClient, "opsmind-test-attachments")

	content := strings.Repeat("测试文件内容\n", 100)
	reader := bytes.NewReader([]byte(content))

	ctx := context.Background()
	key, err := client.Upload(ctx, "opsmind-test-attachments", "test/upload.txt", reader, int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if key != "test/upload.txt" {
		t.Errorf("期望 key='test/upload.txt', got '%s'", key)
	}

	// 验证文件存在
	_, err = rawClient.StatObject(ctx, "opsmind-test-attachments", key, minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("上传后文件应存在: %v", err)
	}

	// 清理
	rawClient.RemoveObject(ctx, "opsmind-test-attachments", key, minio.RemoveObjectOptions{})
}

func TestStorageClient_Upload_EmptyContent(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := adapter.NewMinIOClient(rawClient, "opsmind-test-attachments")

	reader := bytes.NewReader([]byte{})
	ctx := context.Background()
	key, err := client.Upload(ctx, "opsmind-test-attachments", "test/empty.txt", reader, 0, "text/plain")
	if err != nil {
		t.Fatalf("空文件上传不应报错: %v", err)
	}
	_ = key

	// 清理
	rawClient.RemoveObject(ctx, "opsmind-test-attachments", key, minio.RemoveObjectOptions{})
}

// =============================================================================
// GetPresignedURL 测试
// =============================================================================

func TestStorageClient_GetPresignedURL(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := adapter.NewMinIOClient(rawClient, "opsmind-test-presigned")

	// 先上传文件
	content := []byte("预签名测试")
	ctx := context.Background()
	key, err := client.Upload(ctx, "opsmind-test-presigned", "test/presigned.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}

	// 获取预签名 URL
	url, err := client.GetPresignedURL(ctx, "opsmind-test-presigned", key, 1*time.Hour)
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
	rawClient.RemoveObject(ctx, "opsmind-test-presigned", key, minio.RemoveObjectOptions{})
}

// =============================================================================
// Delete 测试
// =============================================================================

func TestStorageClient_Delete(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := adapter.NewMinIOClient(rawClient, "opsmind-test-delete")

	// 先上传文件
	content := []byte("待删除文件")
	ctx := context.Background()
	key, err := client.Upload(ctx, "opsmind-test-delete", "test/to-delete.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}

	// 删除文件
	err = client.Delete(ctx, "opsmind-test-delete", key)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证文件已删除
	_, err = rawClient.StatObject(ctx, "opsmind-test-delete", key, minio.StatObjectOptions{})
	if err == nil {
		t.Error("删除后文件应不存在")
	}
}

func TestStorageClient_Delete_NotFound(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := adapter.NewMinIOClient(rawClient, "opsmind-test-delete")

	// 删除不存在的对象不应报错（幂等性）
	// MinIO RemoveObject 对不存在的 key 返回 204，不会报错
	err = client.Delete(context.Background(), "opsmind-test-delete", "nonexistent/file.txt")
	if err != nil {
		t.Fatalf("删除不存在的对象不应报错（幂等）, got %v", err)
	}
}

// =============================================================================
// 文件读写验证（端到端）
// =============================================================================

func TestStorageClient_UploadDownloadRoundTrip(t *testing.T) {
	rawClient := tryConnectMinIO(t)
	client, err := adapter.NewMinIOClient(rawClient, "opsmind-test-roundtrip")

	original := []byte("端到端测试数据: 你好，世界！")
	ctx := context.Background()

	// 上传
	key, err := client.Upload(ctx, "opsmind-test-roundtrip", "test/roundtrip.txt", bytes.NewReader(original), int64(len(original)), "text/plain")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}

	// 获取预签名 URL
	url, err := client.GetPresignedURL(ctx, "opsmind-test-roundtrip", key, 15*time.Minute)
	if err != nil {
		t.Fatalf("获取预签名失败: %v", err)
	}
	if url == "" {
		t.Error("预签名 URL 不应为空")
	}

	// 通过原始客户端下载验证内容
	obj, err := rawClient.GetObject(ctx, "opsmind-test-roundtrip", key, minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	defer obj.Close()

	downloaded, err := io.ReadAll(obj)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if string(downloaded) != string(original) {
		t.Errorf("内容不一致: 期望 '%s', got '%s'", string(original), string(downloaded))
	}

	// 清理
	rawClient.RemoveObject(ctx, "opsmind-test-roundtrip", key, minio.RemoveObjectOptions{})
}
