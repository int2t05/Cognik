// Package storage — minio.go MinIO 实现 StorageClient 接口（目录式存储）。
package storage

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

// 重试常量。
const (
	defaultMaxRetries = 3
	retryBaseDelay    = 500 * time.Millisecond
)

// MinIOClient 通过 minio-go SDK 实现 StorageClient 接口。
type MinIOClient struct {
	client     *minio.Client
	maxRetries int
}

// NewMinIOClient 创建 MinIOClient 实例，自动确保指定 buckets 存在。
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

// UploadFile 上传单文件到 MinIO（含指数退避重试）。
func (c *MinIOClient) UploadFile(ctx context.Context, bucket, dir, filename string, reader io.Reader, size int64, contentType string) error {
	key := dir + "/" + filename

	buf, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("读取上传内容失败: %w", err)
	}
	if size <= 0 {
		size = int64(len(buf))
	}

	opts := minio.PutObjectOptions{ContentType: contentType}
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
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		_, lastErr = c.client.PutObject(ctx, bucket, key, bytes.NewReader(buf), size, opts)
		if lastErr == nil {
			return nil
		}
	}
	return fmt.Errorf("上传文件失败 [%s/%s] (重试%d次): %w", bucket, key, c.maxRetries, lastErr)
}

// DownloadFile 下载单文件（MinIO GetObject，调用方负责关闭）。
func (c *MinIOClient) DownloadFile(ctx context.Context, bucket, dir, filename string) (io.ReadCloser, error) {
	key := dir + "/" + filename
	obj, err := c.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("下载文件失败 [%s/%s]: %w", bucket, key, err)
	}
	// 触发实际请求以早暴露不存在等错误
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, fmt.Errorf("文件不存在或不可读 [%s/%s]: %w", bucket, key, err)
	}
	return obj, nil
}

// DeleteFile 删除单文件（幂等，MinIO RemoveObject 对不存在对象也返回成功）。
func (c *MinIOClient) DeleteFile(ctx context.Context, bucket, dir, filename string) error {
	key := dir + "/" + filename
	if err := c.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除文件失败 [%s/%s]: %w", bucket, key, err)
	}
	return nil
}

// GetFileURL 生成单文件的预签名下载 URL。
func (c *MinIOClient) GetFileURL(ctx context.Context, bucket, dir, filename string) (string, error) {
	key := dir + "/" + filename
	reqParams := make(url.Values)
	url, err := c.client.PresignedGetObject(ctx, bucket, key, 24*time.Hour, reqParams)
	if err != nil {
		return "", fmt.Errorf("生成预签名 URL 失败 [%s/%s]: %w", bucket, key, err)
	}
	return url.String(), nil
}
