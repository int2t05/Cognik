package runtime

import (
	"context"

	"gorm.io/gorm"
)

// TxManager 事务管理器接口。回调签名用 *gorm.DB，与 Repository 层 GORM 耦合。
type TxManager interface {
	Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}

// GormTxManager 基于 GORM 的 TxManager 实现。
type GormTxManager struct {
	db *gorm.DB
}

// NewGormTxManager 创建事务管理器。db 为 nil 时立即 panic，构造期暴露装配错误。
func NewGormTxManager(db *gorm.DB) *GormTxManager {
	if db == nil {
		panic("cognos: NewGormTxManager called with nil db")
	}
	return &GormTxManager{db: db}
}

// Transaction 在事务中执行 fn。ctx 用于传播请求取消和超时到 DB 查询。
func (m *GormTxManager) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(fn)
}
