// Package tools_test 验证 Agent 内置工具（bash / read_file / write_file / edit_file / list_dir / glob / grep / mkdir）的核心行为。
// 真实文件操作，无 mock。
package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cognik/internal/agent"
	agenttools "cognik/internal/agent/tools"
)

// newWorkDir 创建临时 workDir（替代废弃的 ToolFactory）。
func newWorkDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// runSyncTool 辅助调用 SyncTool.Call（成功路径）。
func runSyncTool(t *testing.T, tool agent.SyncTool, args string) string {
	t.Helper()
	out, err := tool.Call(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	return out
}

// --- read_file ---

func TestReadFile_LineNumbersAndOffset(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	rf := agenttools.NewReadFileTool(dir, 64*1024)
	out := runSyncTool(t, rf, `{"path":"a.txt","offset":2,"limit":2}`)
	if !strings.Contains(out, "     2\tline2") {
		t.Errorf("应含行号 2 + line2，得到:\n%s", out)
	}
	if !strings.Contains(out, "     3\tline3") {
		t.Errorf("应含行号 3 + line3，得到:\n%s", out)
	}
	if strings.Contains(out, "line1") {
		t.Errorf("offset=2 不应含 line1，得到:\n%s", out)
	}
	if strings.Contains(out, "line4") {
		t.Errorf("limit=2 不应含 line4，得到:\n%s", out)
	}
}

func TestReadFile_EmptyFile(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "empty.txt"), []byte(""), 0644)
	rf := agenttools.NewReadFileTool(dir, 64*1024)
	out := runSyncTool(t, rf, `{"path":"empty.txt"}`)
	if !strings.Contains(out, "no content") {
		t.Errorf("空文件应返回 no content 提示，得到:\n%s", out)
	}
}

func TestReadFile_PathTraversalBlocked(t *testing.T) {
	dir := newWorkDir(t)
	rf := agenttools.NewReadFileTool(dir, 64*1024)
	_, err := rf.Call(context.Background(), `{"path":"../../../etc/passwd"}`, nil)
	if err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
}

// --- write_file ---

func TestWriteFile_Overwrite(t *testing.T) {
	dir := newWorkDir(t)
	wf := agenttools.NewWriteFileTool(dir, 64*1024)
	out := runSyncTool(t, wf, `{"path":"f.txt","content":"hello"}`)
	if !strings.Contains(out, "wrote") {
		t.Errorf("应返回 wrote，得到: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(data) != "hello" {
		t.Errorf("文件内容应为 hello，得到 %s", data)
	}
	runSyncTool(t, wf, `{"path":"f.txt","content":"world"}`)
	data, _ = os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(data) != "world" {
		t.Errorf("覆盖后应为 world，得到 %s", data)
	}
}

func TestWriteFile_Append(t *testing.T) {
	dir := newWorkDir(t)
	wf := agenttools.NewWriteFileTool(dir, 64*1024)
	runSyncTool(t, wf, `{"path":"log.txt","content":"line1\n"}`)
	out := runSyncTool(t, wf, `{"path":"log.txt","content":"line2\n","mode":"append"}`)
	if !strings.Contains(out, "appended") {
		t.Errorf("应返回 appended，得到: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "log.txt"))
	if string(data) != "line1\nline2\n" {
		t.Errorf("追加后应为 line1+line2，得到 %q", data)
	}
}

func TestWriteFile_AppendCreateIfMissing(t *testing.T) {
	dir := newWorkDir(t)
	wf := agenttools.NewWriteFileTool(dir, 64*1024)
	out := runSyncTool(t, wf, `{"path":"new.txt","content":"first","mode":"append"}`)
	if !strings.Contains(out, "appended") {
		t.Errorf("append 模式文件不存在应创建，得到: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "new.txt"))
	if string(data) != "first" {
		t.Errorf("应为 first，得到 %s", data)
	}
}

func TestWriteFile_ParentDirCreated(t *testing.T) {
	dir := newWorkDir(t)
	wf := agenttools.NewWriteFileTool(dir, 64*1024)
	runSyncTool(t, wf, `{"path":"sub/dir/f.txt","content":"x"}`)
	data, _ := os.ReadFile(filepath.Join(dir, "sub", "dir", "f.txt"))
	if string(data) != "x" {
		t.Errorf("应自动创建父目录，得到 %s", data)
	}
}

// --- edit_file ---

func TestEditFile_StrReplace(t *testing.T) {
	dir := newWorkDir(t)
	p := filepath.Join(dir, "e.txt")
	os.WriteFile(p, []byte("foo bar baz"), 0644)

	ef := agenttools.NewEditFileTool(dir, 64*1024)
	out := runSyncTool(t, ef, `{"path":"e.txt","old_string":"bar","new_string":"QUX"}`)
	if !strings.Contains(out, "replaced 1") {
		t.Errorf("应替换 1 处，得到: %s", out)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "foo QUX baz" {
		t.Errorf("替换后应为 foo QUX baz，得到 %s", data)
	}
}

func TestEditFile_DeleteByEmptyNew(t *testing.T) {
	dir := newWorkDir(t)
	p := filepath.Join(dir, "e.txt")
	os.WriteFile(p, []byte("keep remove keep"), 0644)

	ef := agenttools.NewEditFileTool(dir, 64*1024)
	runSyncTool(t, ef, `{"path":"e.txt","old_string":"remove ","new_string":""}`)
	data, _ := os.ReadFile(p)
	if string(data) != "keep keep" {
		t.Errorf("删除后应为 keep keep，得到 %s", data)
	}
}

func TestEditFile_ReplaceAll(t *testing.T) {
	dir := newWorkDir(t)
	p := filepath.Join(dir, "e.txt")
	os.WriteFile(p, []byte("x x x"), 0644)

	ef := agenttools.NewEditFileTool(dir, 64*1024)
	out := runSyncTool(t, ef, `{"path":"e.txt","old_string":"x","new_string":"y","replace_all":true}`)
	if !strings.Contains(out, "replaced 3") {
		t.Errorf("应替换 3 处，得到: %s", out)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "y y y" {
		t.Errorf("替换后应为 y y y，得到 %s", data)
	}
}

func TestEditFile_AmbiguousNoReplaceAll(t *testing.T) {
	dir := newWorkDir(t)
	p := filepath.Join(dir, "e.txt")
	os.WriteFile(p, []byte("dup dup"), 0644)

	ef := agenttools.NewEditFileTool(dir, 64*1024)
	_, err := ef.Call(context.Background(), `{"path":"e.txt","old_string":"dup","new_string":"z"}`, nil)
	if err == nil || !strings.Contains(err.Error(), "appears 2 times") {
		t.Fatalf("多处匹配无 replace_all 应报错，得到: %v", err)
	}
}

func TestEditFile_NotFoundFeedback(t *testing.T) {
	dir := newWorkDir(t)
	p := filepath.Join(dir, "e.txt")
	os.WriteFile(p, []byte("line1\nline2\nline3\n"), 0644)

	ef := agenttools.NewEditFileTool(dir, 64*1024)
	_, err := ef.Call(context.Background(), `{"path":"e.txt","old_string":"nonexistent","new_string":"x"}`, nil)
	if err == nil {
		t.Fatal("未匹配应报错")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("错误应含 not found，得到: %v", err)
	}
	if !strings.Contains(err.Error(), "line2") {
		t.Errorf("错误应含文件内容上下文(line2)，得到: %v", err)
	}
}

func TestEditFile_NewStringMustDiffer(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "e.txt"), []byte("same"), 0644)
	ef := agenttools.NewEditFileTool(dir, 64*1024)
	_, err := ef.Call(context.Background(), `{"path":"e.txt","old_string":"same","new_string":"same"}`, nil)
	if err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("相同 old/new 应报错，得到: %v", err)
	}
}

// --- bash ---

func TestBash_Success(t *testing.T) {
	dir := newWorkDir(t)
	b := agenttools.NewBashTool(dir, 5*time.Second, 64*1024)
	out := runSyncTool(t, b, `{"command":"echo hello","description":"test echo"}`)
	if !strings.Contains(out, "exit_code=0") {
		t.Errorf("应 exit_code=0，得到: %s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("应含 hello，得到: %s", out)
	}
}

func TestBash_NonZeroExit(t *testing.T) {
	dir := newWorkDir(t)
	b := agenttools.NewBashTool(dir, 5*time.Second, 64*1024)
	out := runSyncTool(t, b, `{"command":"exit 3","description":"test exit"}`)
	if !strings.Contains(out, "exit_code=3") {
		t.Errorf("应 exit_code=3，得到: %s", out)
	}
}

func TestBash_WorkDirSandbox(t *testing.T) {
	dir := newWorkDir(t)
	b := agenttools.NewBashTool(dir, 5*time.Second, 64*1024)
	out := runSyncTool(t, b, `{"command":"echo sandbox > probe.txt","description":"write probe"}`)
	if !strings.Contains(out, "exit_code=0") {
		t.Errorf("应 exit_code=0，得到: %s", out)
	}
	data, err := os.ReadFile(filepath.Join(dir, "probe.txt"))
	if err != nil {
		t.Fatalf("probe.txt 应出现在 workDir（bash 沙箱），读取失败: %v", err)
	}
	if !strings.Contains(string(data), "sandbox") {
		t.Errorf("probe.txt 内容应含 sandbox，得到 %s", data)
	}
}

func TestBash_Timeout(t *testing.T) {
	dir := newWorkDir(t)
	b := agenttools.NewBashTool(dir, 200*time.Millisecond, 64*1024)
	out := runSyncTool(t, b, `{"command":"sleep 5","description":"test timeout"}`)
	if !strings.Contains(out, "timeout") {
		t.Errorf("超时应返回 timeout 提示，得到: %s", out)
	}
}

// --- list_dir ---

func TestListDir_Entries(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0644)
	os.Mkdir(filepath.Join(dir, "sub"), 0755)

	ld := agenttools.NewListDirTool(dir, 64*1024)
	out := runSyncTool(t, ld, `{"path":"."}`)
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.go") {
		t.Errorf("应列出文件，得到:\n%s", out)
	}
	if !strings.Contains(out, "d") {
		t.Errorf("目录应有 d 标记，得到:\n%s", out)
	}
	subIdx := strings.Index(out, "sub")
	aIdx := strings.Index(out, "a.txt")
	if subIdx < 0 || aIdx < 0 || subIdx > aIdx {
		t.Errorf("目录应排在文件前，得到:\n%s", out)
	}
}

func TestListDir_PathTraversalBlocked(t *testing.T) {
	dir := newWorkDir(t)
	ld := agenttools.NewListDirTool(dir, 64*1024)
	_, err := ld.Call(context.Background(), `{"path":"../../"}`, nil)
	if err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
}

// --- glob ---

func TestGlob_RecursiveMatch(t *testing.T) {
	dir := newWorkDir(t)
	os.MkdirAll(filepath.Join(dir, "src", "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "b.go"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "sub", "c.go"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "d.txt"), []byte("x"), 0644)

	g := agenttools.NewGlobTool(dir, 64*1024)
	out := runSyncTool(t, g, `{"pattern":"**/*.go"}`)
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "src/b.go") || !strings.Contains(out, "src/sub/c.go") {
		t.Errorf("**/*.go 应匹配 3 个 go 文件，得到:\n%s", out)
	}
	if strings.Contains(out, "d.txt") {
		t.Errorf("不应匹配 .txt，得到:\n%s", out)
	}
}

func TestGlob_NoMatch(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)
	g := agenttools.NewGlobTool(dir, 64*1024)
	out := runSyncTool(t, g, `{"pattern":"**/*.go"}`)
	if !strings.Contains(out, "no files") {
		t.Errorf("无匹配应返回 no files，得到: %s", out)
	}
}

// --- grep ---

func TestGrep_ContentMatch(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "f.go"), []byte("package main\n\nfunc foo() {\n\treturn\n}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "g.txt"), []byte("TODO: fix this\n"), 0644)

	gp := agenttools.NewGrepTool(dir, 64*1024)
	out := runSyncTool(t, gp, `{"pattern":"func \\w+\\("}`)
	if !strings.Contains(out, "f.go:3") {
		t.Errorf("应匹配 f.go 第 3 行的 func foo(，得到:\n%s", out)
	}
	if strings.Contains(out, "TODO") {
		t.Errorf("正则 func 不应匹配 TODO 行，得到:\n%s", out)
	}
}

func TestGrep_IgnoreCase(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("Hello World\n"), 0644)

	gp := agenttools.NewGrepTool(dir, 64*1024)
	out := runSyncTool(t, gp, `{"pattern":"hello","ignore_case":true}`)
	if !strings.Contains(out, "a.txt:1") {
		t.Errorf("忽略大小写应匹配 Hello，得到:\n%s", out)
	}
}

func TestGrep_IncludeFilter(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("match\n"), 0644)

	gp := agenttools.NewGrepTool(dir, 64*1024)
	out := runSyncTool(t, gp, `{"pattern":"match","include":"*.go"}`)
	if strings.Contains(out, "b.txt") {
		t.Errorf("include *.go 不应匹配 b.txt，得到:\n%s", out)
	}
	if !strings.Contains(out, "a.go:1") {
		t.Errorf("应匹配 a.go，得到:\n%s", out)
	}
}

func TestGrep_NoMatch(t *testing.T) {
	dir := newWorkDir(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0644)
	gp := agenttools.NewGrepTool(dir, 64*1024)
	out := runSyncTool(t, gp, `{"pattern":"nonexistent"}`)
	if !strings.Contains(out, "no matches") {
		t.Errorf("无匹配应返回 no matches，得到: %s", out)
	}
}

// --- mkdir ---

func TestMkdir_CreateWithParents(t *testing.T) {
	dir := newWorkDir(t)
	m := agenttools.NewMkdirTool(dir)
	out := runSyncTool(t, m, `{"path":"a/b/c"}`)
	if !strings.Contains(out, "created") {
		t.Errorf("应返回 created，得到: %s", out)
	}
	info, err := os.Stat(filepath.Join(dir, "a", "b", "c"))
	if err != nil || !info.IsDir() {
		t.Errorf("目录 a/b/c 应已创建")
	}
}

func TestMkdir_PathTraversalBlocked(t *testing.T) {
	dir := newWorkDir(t)
	m := agenttools.NewMkdirTool(dir)
	_, err := m.Call(context.Background(), `{"path":"../../escape"}`, nil)
	if err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
}

// --- build（沙箱目录确保）---

// TestBuild_CreatesSandboxIfMissing 验证 Build 自动创建不存在的沙箱工作目录。
// 修复缺陷：全新部署下 workDir 缺失，首次 bash/list_dir 调用直接失败。
func TestBuild_CreatesSandboxIfMissing(t *testing.T) {
	root := t.TempDir()
	sandbox := filepath.Join(root, "nested", "agent-workspace")
	if _, err := os.Stat(sandbox); !os.IsNotExist(err) {
		t.Fatalf("前置条件失败：沙箱目录不应存在, got %v", err)
	}

	tools, err := agenttools.Build(agenttools.Deps{WorkDir: sandbox, Timeout: time.Second, MaxBytes: 1024})
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	if len(tools) == 0 {
		t.Fatal("Build 应返回工具集")
	}
	info, err := os.Stat(sandbox)
	if err != nil || !info.IsDir() {
		t.Errorf("Build 应自动创建沙箱目录 %s, stat err=%v", sandbox, err)
	}
}
