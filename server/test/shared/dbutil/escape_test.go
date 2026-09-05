// Package dbutil_test 验证 SQL 工具函数。
package dbutil_test

import (
	"testing"

	"cognik/internal/shared/pkg/dbutil"
)

// TestEscapeLike 验证 LIKE/ILIKE 通配符转义：%、_、\ 均转义，普通文本不变。
func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name string
		input string
		want  string
	}{
		{"普通文本", "hello", "hello"},
		{"百分号", "100%", `100\%`},
		{"下划线", "a_b", `a\_b`},
		{"反斜杠", `path\to`, `path\\to`},
		{"混合", `a\b%c_d`, `a\\b\%c\_d`},
		{"空串", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dbutil.EscapeLike(tt.input); got != tt.want {
				t.Errorf("EscapeLike(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
