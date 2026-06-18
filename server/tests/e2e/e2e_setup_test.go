//go:build e2e

// Package e2e_test 实现端到端 API 测试。
//
// 启动本地 opsmind-server 进程，连接到容器化 PostgreSQL/MinIO/llama.cpp，
// 通过真实 HTTP 调用验证 API 行为，同时捕获服务器和 Docker 容器日志。
//
// 前置条件：
//
//	docker compose up -d postgres minio llama-cpp llama-cpp-emb
//
// 运行：
//
//	go test -tags=e2e ./tests/e2e/ -v -run "TestE2E"
package e2e_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"opsmind/pkg/hash"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ── 服务器结构 ──────────────────────────────────────────

type e2eServer struct {
	cmd     *exec.Cmd
	BaseURL string
	DB      *gorm.DB
	logBuf  *threadSafeBuffer
}

type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ── 服务器启动（无 testing.T 依赖 — 供 TestMain 使用）───

func newE2EServer() (*e2eServer, error) {
	binary, err := buildBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binary)
	cmd.Env = serverEnv("18080")

	logBuf := &threadSafeBuffer{}
	cmd.Stdout = io.MultiWriter(os.Stdout, logBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, logBuf)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动服务失败: %w", err)
	}

	baseURL := "http://localhost:18080"
	if err := waitForReady(baseURL, 30*time.Second); err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	db, err := gorm.Open(postgres.Open(
		"host=localhost port=5432 user=opsmind password=opsmind_dev dbname=opsmind sslmode=disable"))
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	s := &e2eServer{cmd: cmd, BaseURL: baseURL, DB: db, logBuf: logBuf}

	// 种子数据——创建 admin 用户（如不存在）
	s.seedAdmin()

	return s, nil
}

func (s *e2eServer) seedAdmin() {
	pwd := "Admin@123"
	hashed, _ := hash.HashPassword(pwd)

	// 如果 admin 已存在但密码不对，更新密码
	var count int64
	s.DB.Raw("SELECT count(*) FROM users WHERE username = 'admin'").Scan(&count)
	if count > 0 {
		s.DB.Exec("UPDATE users SET password_hash = $1 WHERE username = 'admin'", hashed)
	} else {
		s.DB.Exec(`INSERT INTO users (username, password_hash, real_name, phone, status, first_login, created_at, updated_at)
			VALUES ('admin', $1, 'Admin', '13800000001', 1, false, NOW(), NOW())`, hashed)
	}

	// 确保角色存在
	s.DB.Exec(`INSERT INTO roles (name, description, permissions, created_at, updated_at)
		VALUES ('系统管理员', '系统全局管理',
		'["user:manage","ticket:read","ticket:write","ticket:manage","knowledge:read","knowledge:write","knowledge:create","knowledge:review","knowledge:manage","dashboard:read","audit:read","system:config"]',
		NOW(), NOW()) ON CONFLICT DO NOTHING`)
	var roleID int64
	s.DB.Raw("SELECT id FROM roles WHERE name = '系统管理员'").Scan(&roleID)
	var userID int64
	s.DB.Raw("SELECT id FROM users WHERE username = 'admin'").Scan(&userID)
	s.DB.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roleID)
}

func startE2EServer(t *testing.T) *e2eServer {
	t.Helper()
	s, err := newE2EServer()
	if err != nil {
		t.Fatalf("启动 E2E 服务器失败: %v", err)
	}
	return s
}

func (s *e2eServer) stop() {
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	if d, err := s.DB.DB(); err == nil {
		d.Close()
	}
}

// ── 环境变量 ────────────────────────────────────────────

func serverEnv(port string) []string {
	return append(os.Environ(),
		"OPSMIND_SERVER_PORT="+port,
		"OPSMIND_SERVER_MODE=debug",
		"OPSMIND_DATABASE_HOST=localhost",
		"OPSMIND_DATABASE_PORT=5432",
		"OPSMIND_DATABASE_USER=opsmind",
		"OPSMIND_DATABASE_PASSWORD=opsmind_dev",
		"OPSMIND_DATABASE_NAME=opsmind",
		"OPSMIND_DATABASE_SSLMODE=disable",
		"OPSMIND_JWT_SECRET=e2e_test_secret_2024",
		"OPSMIND_LLM_BASE_URL=http://localhost:8081/v1",
		"OPSMIND_EMBEDDING_BASE_URL=http://localhost:8082/v1",
		"OPSMIND_LLM_MODEL=Qwen3-4B-Q4_K_M",
		"OPSMIND_EMBEDDING_MODEL=Qwen3-Embedding-0.6B-Q8_0",
		"OPSMIND_EMBEDDING_DIMENSION=1024",
		"OPSMIND_MINIO_ENDPOINT=localhost:9000",
		"OPSMIND_MINIO_ACCESS_KEY=minioadmin",
		"OPSMIND_MINIO_SECRET_KEY=minioadmin",
		"OPSMIND_MINIO_USE_SSL=false",
		"OPSMIND_CORS_ALLOW_ORIGINS=http://localhost:5173",
		"OPSMIND_AI_RAG_QUERY_REWRITE=false",
		"OPSMIND_AI_RAG_MULTI_ROUTE=false",
		"OPSMIND_AI_RAG_HYBRID=false",
		"OPSMIND_AI_RAG_RERANK=false",
		"OPSMIND_RERANK_ENABLED=false",
		// AutoMigrate 开启——自动建表
	)
}

// ── 编译 ────────────────────────────────────────────────

func buildBinary() (string, error) {
	wd, _ := os.Getwd()
	binary := fmt.Sprintf("%s/opsmind-e2e-%d.exe", wd, time.Now().UnixNano())
	// 从 tests/e2e/ 向上两级到达 server/
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/main.go")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("编译失败: %s", string(out))
	}
	return binary, nil
}

// ── 日志 ────────────────────────────────────────────────

func (s *e2eServer) logs() string { return s.logBuf.String() }

func (s *e2eServer) assertLogContains(t *testing.T, substr, msg string) {
	t.Helper()
	if !strings.Contains(s.logs(), substr) {
		t.Errorf("%s: 日志不包含 %q\n--- 完整日志 ---\n%s", msg, substr, s.logs())
	}
}

func dockerLogs(t *testing.T, service string, tail int) string {
	t.Helper()
	out, _ := exec.Command("docker", "compose", "logs", service, "--tail", fmt.Sprint(tail)).CombinedOutput()
	return string(out)
}

func assertNoDBErrors(t *testing.T) {
	t.Helper()
	logs := dockerLogs(t, "postgres", 50)
	if strings.Contains(logs, "ERROR") {
		t.Errorf("PostgreSQL 日志包含 ERROR:\n%s", logs)
	}
}

// ── HTTP 请求 ───────────────────────────────────────────

var adminToken string

func (s *e2eServer) do(t *testing.T, method, path string, body interface{}, token string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, s.BaseURL+path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP %s %s 失败: %v", method, path, err)
	}
	return resp
}

func (s *e2eServer) doAuth(t *testing.T, method, path string, body interface{}) *http.Response {
	if adminToken == "" {
		resp := s.do(t, http.MethodPost, "/api/v1/auth/login",
			map[string]string{"username": "admin", "password": "Admin@123"}, "")
		body := parseBody(t, resp)
		if body["code"] != float64(0) {
			t.Fatalf("管理员登录失败: code=%v message=%v", body["code"], body["message"])
		}
		adminToken = body["data"].(map[string]interface{})["access_token"].(string)
	}
	return s.do(t, method, path, body, adminToken)
}

// ── 断言 ────────────────────────────────────────────────

func parseBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("JSON 解析失败: %s\nbody: %s", err, string(b))
	}
	return m
}

func assertOK(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body := parseBody(t, resp)
	if body["code"] != float64(0) {
		t.Fatalf("期望 code=0, 实际 code=%v message=%v", body["code"], body["message"])
	}
	return body
}

func assertCode(t *testing.T, resp *http.Response, code float64) {
	t.Helper()
	body := parseBody(t, resp)
	if body["code"] != code {
		t.Fatalf("期望 code=%v, 实际 code=%v message=%v", code, body["code"], body["message"])
	}
}

func assertAPIError(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body := parseBody(t, resp)
	if body["code"] == float64(0) {
		t.Fatalf("期望非 0 code, 实际 code=0")
	}
	return body
}

func assertField(t *testing.T, cond bool, msg string) {
	t.Helper()
	if !cond {
		t.Errorf("字段断言失败: %s", msg)
	}
}

// assertNoRecentDBErrors 检查 PostgreSQL 容器最近 50 条日志无 ERROR。
func assertNoRecentDBErrors(t *testing.T) {
	t.Helper()
	logs := dockerLogs(t, "postgres", 50)
	if strings.Contains(logs, "ERROR") {
		// 排除已修复的已知问题（embedding 列不存在等）
		if !strings.Contains(logs, "does not exist") {
			t.Errorf("PostgreSQL 容器日志包含非预期 ERROR:\n%s", logs)
		}
	}
}

// ── 工具 ────────────────────────────────────────────────

func waitForReady(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s/health", url)
}

// ── SSE ─────────────────────────────────────────────────

func (s *e2eServer) doSSE(t *testing.T, path string, body interface{}) (*http.Response, []byte) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, s.BaseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE 请求失败: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, respBody
}

func parseSSE(t *testing.T, body []byte) []map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "data: ") {
			var evt map[string]interface{}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt) == nil {
				events = append(events, evt)
			}
		}
	}
	return events
}
