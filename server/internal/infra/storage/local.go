// Package storage — local.go 本地文件系统存储实现。
// 目录式存储：每篇文章存储为 {baseDir}/{bucket}/{dir}/ 下的多文件目录。
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

// DownloadDir 下载整个目录，返回 filename→reader 映射。
// 调用方负责关闭每个 reader。
func (c *LocalStorageClient) DownloadDir(ctx context.Context, bucket, dir string) (map[string]io.ReadCloser, error) {
	dirPath := filepath.Join(c.baseDir, bucket, dir)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("目录不存在 [%s/%s]", bucket, dir)
		}
		return nil, fmt.Errorf("读取目录失败 [%s/%s]: %w", bucket, dir, err)
	}

	result := make(map[string]io.ReadCloser)

	// 递归收集所有文件（含 images/ 子目录）
	var collect func(base string, entries []os.DirEntry) error
	collect = func(base string, entries []os.DirEntry) error {
		for _, entry := range entries {
			fullPath := filepath.Join(base, entry.Name())
			if entry.IsDir() {
				subEntries, err := os.ReadDir(fullPath)
				if err != nil {
					return fmt.Errorf("读取子目录失败 %s: %w", fullPath, err)
				}
				if err := collect(fullPath, subEntries); err != nil {
					return err
				}
				continue
			}
			f, err := os.Open(fullPath)
			if err != nil {
				return fmt.Errorf("打开文件失败 %s: %w", fullPath, err)
			}
			// 相对路径（相对 dirPath），如 markdown.md / images/xxx.jpg
			rel, _ := filepath.Rel(dirPath, fullPath)
			result[filepath.ToSlash(rel)] = f
		}
		return nil
	}

	if err := collect(dirPath, entries); err != nil {
		// 清理已打开的 reader
		for _, r := range result {
			r.Close()
		}
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("目录为空 [%s/%s]", bucket, dir)
	}
	return result, nil
}

// DeleteDir 删除整个目录（递归，幂等）。
func (c *LocalStorageClient) DeleteDir(ctx context.Context, bucket, dir string) error {
	dirPath := filepath.Join(c.baseDir, bucket, dir)

	if err := os.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("删除目录失败 [%s/%s]: %w", bucket, dir, err)
	}
	return nil
}

// GetFileURL 返回本地文件路径。
func (c *LocalStorageClient) GetFileURL(ctx context.Context, bucket, dir, filename string) (string, error) {
	return filepath.Join(c.baseDir, bucket, dir, filename), nil
}
