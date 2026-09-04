// Package storage — local.go 本地文件系统存储实现（目录式）。
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorageClient 通过本地文件系统实现 StorageClient 接口。
type LocalStorageClient struct {
	baseDir string
}

// NewLocalStorageClient 创建 LocalStorageClient，自动确保 bucket 目录存在。
func NewLocalStorageClient(baseDir string, buckets ...string) (*LocalStorageClient, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base_dir 不能为空")
	}

	for _, bucket := range buckets {
		dir := filepath.Join(baseDir, bucket)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建存储目录失败 %s: %w", dir, err)
		}
	}

	return &LocalStorageClient{baseDir: baseDir}, nil
}

// UploadFile 上传单文件到本地文件系统。
func (c *LocalStorageClient) UploadFile(ctx context.Context, bucket, dir, filename string, reader io.Reader, size int64, contentType string) error {
	path := filepath.Join(c.baseDir, bucket, dir, filename)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", filepath.Dir(path), err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建文件失败 %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("写入文件失败 %s: %w", path, err)
	}
	return nil
}

// DownloadFile 下载单文件（本地直接打开，调用方负责关闭）。
func (c *LocalStorageClient) DownloadFile(ctx context.Context, bucket, dir, filename string) (io.ReadCloser, error) {
	path := filepath.Join(c.baseDir, bucket, dir, filename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("文件不存在 [%s/%s/%s]", bucket, dir, filename)
		}
		return nil, fmt.Errorf("打开文件失败 %s: %w", path, err)
	}
	return f, nil
}

// DeleteFile 删除单文件（幂等，文件不存在不报错）。
func (c *LocalStorageClient) DeleteFile(ctx context.Context, bucket, dir, filename string) error {
	path := filepath.Join(c.baseDir, bucket, dir, filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除文件失败 %s: %w", path, err)
	}
	return nil
}

// GetFileURL 返回本地文件路径。
func (c *LocalStorageClient) GetFileURL(ctx context.Context, bucket, dir, filename string) (string, error) {
	return filepath.Join(c.baseDir, bucket, dir, filename), nil
}
