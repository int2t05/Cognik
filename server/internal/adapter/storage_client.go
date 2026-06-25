// Package adapter 提供外部服务的适配层。
//
// storage_client.go 定义 StorageClient 接口和 MinIO 实现。
// 所有 MinIO 对象存储操作必须通过此适配层，禁止直接 HTTP 调用。
//
// 接口包含 3 个方法：Upload / GetPresignedURL / Delete。
// Bucket 规划：opsmind-attachments（申告附件）、opsmind-documents（知识文档）。
// 初始化时自动创建 bucket（MakeBucket）。
package adapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

// =============================================================================
// 接口定义
// =============================================================================

// StorageClient 定义对象存储适配器接口。
//
// TicketService 和 KnowledgeService 依赖此接口完成附件上传和文档管理。
// 为什么定义为接口：后续可替换为其他 S3-compatible 存储（如 AWS S3、阿里云 OSS）
// 而不影响上层业务逻辑。
type StorageClient interface {
	// Upload 上传文件到指定 bucket，返回对象 key。
	//
	// key 建议格式："{resource_type}/{timestamp}_{filename}"，如 "tickets/20260610_attach.pdf"
	Upload(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (string, error)

	// Download 从指定 bucket 下载对象。
	//
	// 返回的 io.ReadCloser 由调用方负责关闭。
	// 对象不存在时返回错误。
	Download(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// GetPresignedURL 获取对象的预签名下载 URL，expiry 为有效期。
	//
	// 预签名 URL 可直接通过 HTTP GET 访问，无需认证。
	// 典型用途：返回给前端直接下载附件。
	GetPresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)

	// Delete 从指定 bucket 删除对象。
	//
	// 找不到对象时不报错（幂等性），因为删除操作的目标状态是"对象不存在"。
	Delete(ctx context.Context, bucket, key string) error
}

// =============================================================================
// MinIO 实现
// =============================================================================

// MinIOClient 通过 minio-go SDK 实现 StorageClient 接口。
type MinIOClient struct {
	client     *minio.Client
	maxRetries int
}

// NewMinIOClient 创建 MinIOClient 实例，自动确保指定 buckets 存在。
//
// buckets 参数为需确保存在的 bucket 列表（如 "opsmind-attachments", "opsmind-documents"）。
// 为什么在构造函数中确保 bucket 存在：应用启动时一次性创建，避免每个请求都检查。
// ensureBucket 任一失败则返回 error，阻止不带存储能力的服务启动。
func NewMinIOClient(client *minio.Client, buckets ...string) (*MinIOClient, error) {
	mc := &MinIOClient{client: client, maxRetries: defaultMaxRetries}

	for _, bucket := range buckets {
		if err := mc.ensureBucket(context.Background(), bucket); err != nil {
			return nil, fmt.Errorf("ensureBucket %s 失败: %w", bucket, err)
		}
	}

	return mc, nil
}

// ensureBucket 确保 bucket 存在，不存在则创建。
func (c *MinIOClient) ensureBucket(ctx context.Context, bucket string) error {
	exists, err := c.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("检查 bucket %s 失败: %w", bucket, err)
	}
	if !exists {
		if err := c.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("创建 bucket %s 失败: %w", bucket, err)
		}
	}
	return nil
}

// =============================================================================
// Upload
// =============================================================================

// Upload 上传文件到 MinIO（含指数退避重试）。
//
// 使用 PutObject API，自动检测 Content-Type。
// 返回与入参相同的 key，方便调用方确认。
//
// io.Reader 会被全部读入内存缓冲，以支持重试（io.Reader 不可回退）。
// 对超大文件需注意内存占用——当前 OpsMind 上传上限为 50MB。
func (c *MinIOClient) Upload(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (string, error) {
	// 将 reader 内容缓冲到内存，保证重试时每次都能从起始位置读取
	buf, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取上传内容失败: %w", err)
	}
	if size <= 0 {
		size = int64(len(buf))
	}

	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}
	if contentType == "" {
		opts.ContentType = "application/octet-stream"
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			if delay > 8*time.Second {
				delay = 8 * time.Second
			}
			slog.Warn("MinIO 上传重试中", "bucket", bucket, "key", key, "attempt", attempt, "delay_ms", delay.Milliseconds())
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		_, lastErr = c.client.PutObject(ctx, bucket, key, bytes.NewReader(buf), size, opts)
		if lastErr == nil {
			return key, nil
		}
	}
	return "", fmt.Errorf("上传文件失败 [%s/%s] (重试%d次): %w", bucket, key, c.maxRetries, lastErr)
}

// =============================================================================
// Download
// =============================================================================

// Download 从 MinIO 下载对象（含指数退避重试）。
//
// 使用 GetObject API，返回 io.ReadCloser。
// 调用方负责在读取完毕后调用 Close() 释放连接。
func (c *MinIOClient) Download(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			if delay > 8*time.Second {
				delay = 8 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		obj, err := c.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
		if err == nil {
			return obj, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("下载文件失败 [%s/%s] (重试%d次): %w", bucket, key, c.maxRetries, lastErr)
}

// =============================================================================
// GetPresignedURL
// =============================================================================

// GetPresignedURL 生成对象的预签名下载 URL。
//
// 使用 PresignedGetObject API，生成有时效的下载链接。
// expiry 建议：附件 1 小时，文档 24 小时。
func (c *MinIOClient) GetPresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	url, err := c.client.PresignedGetObject(ctx, bucket, key, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("生成预签名 URL 失败 [%s/%s]: %w", bucket, key, err)
	}
	return url.String(), nil
}

// =============================================================================
// Delete
// =============================================================================

// Delete 从 MinIO 删除对象。
//
// 使用 RemoveObject API。对象不存在时不报错（MinIO 默认行为即幂等，
// 返回 204 No Content 无论对象是否存在）。
func (c *MinIOClient) Delete(ctx context.Context, bucket, key string) error {
	if err := c.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除文件失败 [%s/%s]: %w", bucket, key, err)
	}
	return nil
}
