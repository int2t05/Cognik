// Package dbutil 提供 SQL 相关的工具函数。
package dbutil

import "strings"

// EscapeLike 转义 LIKE/ILIKE 模式中的通配符（%、_、\），配合 ESCAPE '\\' 子句实现字面量搜索。
//
// 用法：like := "%" + EscapeLike(keyword) + "%"
//
//	query.Where("name ILIKE ? ESCAPE '\\'", like)
func EscapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
