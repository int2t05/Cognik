// Package storage 定义文件存储适配接口（MinIO/本地双实现）。
package storage

import (
	"context"
	"io"
)

// StorageClient 文件存储适配接口（MinIO/本地双实现）。
// path 语义：bucket/{dir}/{filename}，dir 可含子目录（如 "kb-1/draft"）；扁平单文件场景 dir 即文件所在目录。
type StorageClient interface {
	// UploadFile 上传单文件到 bucket/{dir}/{filename}。
	UploadFile(ctx context.Context, bucket, dir, filename string, reader io.Reader, size int64, contentType string) error

	// DownloadFile 下载单文件（bucket/{dir}/{filename}），调用方负责关闭 reader。
	DownloadFile(ctx context.Context, bucket, dir, filename string) (io.ReadCloser, error)

	// DeleteFile 删除单文件（幂等，文件不存在不报错）。
	DeleteFile(ctx context.Context, bucket, dir, filename string) error

	// GetFileURL 获取单文件访问 URL（MinIO 预签名 / 本地路径）。
	GetFileURL(ctx context.Context, bucket, dir, filename string) (string, error)
}
