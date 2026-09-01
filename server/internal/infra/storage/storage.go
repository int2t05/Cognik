// Package storage 定义文件存储适配接口（目录式存储）。
// 实现：minio.go（MinIO S3）、local.go（本地文件系统）。
package storage

import (
	"context"
	"io"
)

// StorageClient 定义对象存储适配器接口（目录式）。
// 每篇文章存储为 bucket/{dir}/ 下的多文件目录（markdown.md + images/）。
type StorageClient interface {
	// UploadFile 上传单文件到 bucket/{dir}/{filename}。
	UploadFile(ctx context.Context, bucket, dir, filename string, reader io.Reader, size int64, contentType string) error

	// DownloadDir 下载整个目录，返回 filename→reader 映射。
	// 调用方负责关闭每个 reader。
	DownloadDir(ctx context.Context, bucket, dir string) (map[string]io.ReadCloser, error)

	// DeleteDir 删除整个目录（递归，幂等）。
	DeleteDir(ctx context.Context, bucket, dir string) error

	// GetFileURL 获取单文件访问 URL（MinIO 预签名 / 本地路径）。
	GetFileURL(ctx context.Context, bucket, dir, filename string) (string, error)
}
