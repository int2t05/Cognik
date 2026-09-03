// Package tools 提供 Agent 内置工具集。
// platform.go：平台相关的 bash 二进制探测。
//
// Windows 默认用 GitBash（对齐开发环境）；可通过 OPSMIND_AGENT_BASH_BIN 覆盖。
// 非 Windows 直接用 PATH 中的 bash/sh。
package tools

import (
	"os"
	"os/exec"
	"runtime"
)

// resolveBashBin 解析要执行的 bash 二进制路径。
// 优先级：OPSMIND_AGENT_BASH_BIN env > Windows GitBash 常见路径 > PATH(bash) > PATH(sh)。
func resolveBashBin() string {
	// 1. 显式配置
	if bin := os.Getenv("OPSMIND_AGENT_BASH_BIN"); bin != "" {
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
	}
	// 2. Windows 探测 GitBash 常见安装路径
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			`C:\Program Files\Git\bin\bash.exe`,
			`C:\Program Files\Git\usr\bin\bash.exe`,
			`C:\Program Files (x86)\Git\bin\bash.exe`,
			`C:\ProgramData\Git\bin\bash.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	// 3. PATH 查找 bash，再 fallback sh
	if p, err := exec.LookPath("bash"); err == nil {
		return p
	}
	return "sh" // POSIX 兜底（所有平台都有）
}
