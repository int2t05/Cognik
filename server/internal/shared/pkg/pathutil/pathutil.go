// Package pathutil 提供存储路径处理工具函数。
package pathutil

import "strings"

// SplitFileKey 把完整 fileKey（如 kb-1/draft/article-2.md）拆为 dir（kb-1/draft）与 filename（article-2.md）。
// 无 "/" 时返回空 dir 与原字符串作为 filename。
func SplitFileKey(fileKey string) (dir, filename string) {
	idx := strings.LastIndex(fileKey, "/")
	if idx < 0 {
		return "", fileKey
	}
	return fileKey[:idx], fileKey[idx+1:]
}
