//go:build e2e

package e2e_test

import (
	"log"
	"os"
	"testing"
)

var e2e *e2eServer

func TestMain(m *testing.M) {
	var err error
	e2e, err = newE2EServer()
	if err != nil {
		log.Printf("启动 E2E 服务器失败: %v", err)
		os.Exit(1)
	}

	code := m.Run()

	e2e.stop()
	os.Exit(code)
}
