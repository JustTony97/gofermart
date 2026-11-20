package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type TxManager struct {
	db *sqlx.DB
}

func NewTxManager(db *sqlx.DB) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) BeginFunc(ctx context.Context, fn func(*sqlx.Tx) error) (err error) {
	tx, err := m.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
		if err != nil {
			tx.Rollback()
			return
		}
		if cmErr := tx.Commit(); cmErr != nil {
			err = fmt.Errorf("commit error: %w", cmErr)
		}
	}()

	err = fn(tx)
	return err
}
