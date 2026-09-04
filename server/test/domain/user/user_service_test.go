//go:build integration

// Package service_test 验证 UserService 业务逻辑。
package user_test

import (
	"testing"

	"cognos/internal/domain/system/audit"
	"cognos/internal/domain/user/account"
	"cognos/internal/infra/config"
	"cognos/internal/infra/database"
	"cognos/internal/shared/model"
	"cognos/internal/shared/pkg/errcode"

	"gorm.io/gorm"
)

var userSvcDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "cognos",
		Password: "cognos_dev",
		DBName:   "cognos_test",
		SSLMode:  "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	userSvcDB = db
}

func setupUserService(t *testing.T) (*account.UserService, *model.User) {
	t.Helper()
	repo := account.NewUserRepo(userSvcDB)
	auditRepo := audit.NewAuditRepo(userSvcDB)
	svc := account.NewUserService(repo, audit.NewAuditService(auditRepo), userSvcDB, nil)

	// 创建测试用户
	user := &model.User{
		Username:     "test_svcuser_1",
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		RealName:     "测试用户",
		Status:       1,
	}
	userSvcDB.Where("username = ?", user.Username).Delete(&model.User{})
	userSvcDB.Create(user)

	return svc, user
}

func TestUserService_GetByID_Success(t *testing.T) {
	svc, user := setupUserService(t)

	result, err := svc.GetByID(bgCtx, user.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Username != "test_svcuser_1" {
		t.Errorf("期望用户名 test_svcuser_1, got %s", result.Username)
	}
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	svc, _ := setupUserService(t)

	_, err := svc.GetByID(bgCtx, 999999)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
	if code := err.(errcode.AppError).Code; code != 10004 {
		t.Errorf("期望错误码 10004, got %d", code)
	}
}

func TestUserService_List_Success(t *testing.T) {
	svc, _ := setupUserService(t)

	result, err := svc.List(bgCtx, 1, 10, "")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Total < 1 {
		t.Errorf("期望 total >= 1, got %d", result.Total)
	}
	if len(result.Users) < 1 {
		t.Errorf("期望至少1条记录, got %d", len(result.Users))
	}
}

func TestUserService_List_WithKeyword(t *testing.T) {
	svc, _ := setupUserService(t)

	result, err := svc.List(bgCtx, 1, 10, "test_svcuser_1")
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if len(result.Users) < 1 {
		t.Errorf("期望至少1条记录, got %d", len(result.Users))
	}
}

func TestUserService_Freeze_Success(t *testing.T) {
	svc, user := setupUserService(t)

	err := svc.Freeze(bgCtx, user.ID, 1)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.User
	userSvcDB.First(&updated, user.ID)
	if updated.Status != 2 {
		t.Errorf("期望状态 2(frozen), got %d", updated.Status)
	}
}

func TestUserService_Freeze_AlreadyFrozen(t *testing.T) {
	svc, user := setupUserService(t)
	userSvcDB.Model(user).Update("status", 2)

	err := svc.Freeze(bgCtx, user.ID, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
	if code := err.(errcode.AppError).Code; code != 10006 {
		t.Errorf("期望错误码 10006, got %d", code)
	}
}

func TestUserService_Freeze_NotFound(t *testing.T) {
	svc, _ := setupUserService(t)

	err := svc.Freeze(bgCtx, 999999, 1)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}

func TestUserService_Restore_Success(t *testing.T) {
	svc, user := setupUserService(t)
	userSvcDB.Model(user).Update("status", 2)

	err := svc.Restore(bgCtx, user.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.User
	userSvcDB.First(&updated, user.ID)
	if updated.Status != 1 {
		t.Errorf("期望状态 1(active), got %d", updated.Status)
	}
}

func TestUserService_Restore_AlreadyActive(t *testing.T) {
	svc, user := setupUserService(t)

	err := svc.Restore(bgCtx, user.ID)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
	if code := err.(errcode.AppError).Code; code != 10007 {
		t.Errorf("期望错误码 10007, got %d", code)
	}
}

func TestUserService_Restore_NotFound(t *testing.T) {
	svc, _ := setupUserService(t)

	err := svc.Restore(bgCtx, 999999)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
}
