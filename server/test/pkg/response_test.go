// Package pkg_test 测试公共工具包的导出 API。
//
// 采用外部测试包（black-box testing），只测试公开接口，
// 确保测试覆盖的是用户可见的行为而非内部实现。
package pkg_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"opsmind/pkg/response"
)

// setupGinContext 创建一个用于测试的 gin.Context
func setupGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)
	return c, w
}

// TestSuccess 测试成功响应格式
func TestSuccess(t *testing.T) {
	c, w := setupGinContext()

	// 调用 Success
	response.Success(c, map[string]string{"name": "test"})

	// 验证 HTTP 状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 解析响应体
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应体失败: %v", err)
	}

	// 验证 code 字段
	if code, ok := resp["code"].(float64); !ok || code != 0 {
		t.Errorf("期望 code=0，实际 %v", resp["code"])
	}

	// 验证 message 字段
	if msg, ok := resp["message"].(string); !ok || msg != "success" {
		t.Errorf("期望 message=\"success\"，实际 %v", resp["message"])
	}

	// 验证 data 字段
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 字段不是对象: %v", resp["data"])
	}
	if data["name"] != "test" {
		t.Errorf("期望 data.name=\"test\"，实际 %v", data["name"])
	}
}

// TestSuccessWithNil 测试 data 为 nil 时的响应
func TestSuccessWithNil(t *testing.T) {
	c, w := setupGinContext()

	// 调用 Success，data 为 nil
	response.Success(c, nil)

	// 解析响应体
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应体失败: %v", err)
	}

	// 验证 data 字段存在且为 null
	if _, exists := resp["data"]; !exists {
		t.Error("data 字段应该存在")
	}
	if resp["data"] != nil {
		t.Errorf("期望 data=null，实际 %v", resp["data"])
	}
}

// TestError 测试错误响应格式
func TestError(t *testing.T) {
	c, w := setupGinContext()

	// 调用 Error
	response.Error(c, 10001, "未登录或令牌过期")

	// 验证 HTTP 状态码
	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusUnauthorized, w.Code)
	}

	// 解析响应体
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应体失败: %v", err)
	}

	// 验证 code 字段
	if code, ok := resp["code"].(float64); !ok || code != 10001 {
		t.Errorf("期望 code=10001，实际 %v", resp["code"])
	}

	// 验证 message 字段
	if msg, ok := resp["message"].(string); !ok || msg != "未登录或令牌过期" {
		t.Errorf("期望 message=\"未登录或令牌过期\"，实际 %v", resp["message"])
	}

	// 验证 data 字段为 null
	if resp["data"] != nil {
		t.Errorf("期望 data=null，实际 %v", resp["data"])
	}
}

// TestErrorWithDifferentCodes 测试不同错误码映射正确的 HTTP 状态
func TestErrorWithDifferentCodes(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		httpStatus int
	}{
		{"未登录", 10001, 401},
		{"无权限", 10002, 403},
		{"参数错误", 10003, 400},
		{"资源不存在", 10004, 404},
		{"资源冲突", 10005, 409},
		{"AI服务不可用", 20001, 503},
		{"RAG服务不可用", 20002, 503},
		{"存储服务不可用", 20003, 503},
		{"未知错误", 99999, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupGinContext()
			response.Error(c, tt.code, tt.name)

			if w.Code != tt.httpStatus {
				t.Errorf("错误码 %d: 期望 HTTP %d，实际 %d", tt.code, tt.httpStatus, w.Code)
			}
		})
	}
}

// TestSuccessWithPage 测试分页响应格式
func TestSuccessWithPage(t *testing.T) {
	c, w := setupGinContext()

	// 调用 SuccessWithPage
	items := []string{"item1", "item2"}
	response.SuccessWithPage(c, items, 100, 1, 10)

	// 解析响应体
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应体失败: %v", err)
	}

	// 验证 code
	if code, ok := resp["code"].(float64); !ok || code != 0 {
		t.Errorf("期望 code=0，实际 %v", resp["code"])
	}

	// 验证 data 是数组
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("data 字段不是数组: %T %v", resp["data"], resp["data"])
	}
	if len(data) != 2 {
		t.Errorf("期望 data 长度 2，实际 %d", len(data))
	}

	// 验证分页字段
	if total, ok := resp["total"].(float64); !ok || total != 100 {
		t.Errorf("期望 total=100，实际 %v", resp["total"])
	}
	if page, ok := resp["page"].(float64); !ok || page != 1 {
		t.Errorf("期望 page=1，实际 %v", resp["page"])
	}
	if pageSize, ok := resp["page_size"].(float64); !ok || pageSize != 10 {
		t.Errorf("期望 page_size=10，实际 %v", resp["page_size"])
	}
}
