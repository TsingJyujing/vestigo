package utils

import (
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

func WithTx[T any](
	ctx context.Context,
	db *sql.DB,
	opts *sql.TxOptions,
	fn func(tx *sql.Tx) (T, error),
) (out T, err error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return out, err
	}

	// Rollback the transaction on panic or error
	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				logger.Errorf("transaction rollback error: %v", rollbackErr)
			}
			panic(p)
		}
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				logger.Errorf("transaction rollback error: %v", rollbackErr)
			}
		}
	}()

	out, err = fn(tx)
	if err != nil {
		return out, err
	}

	// 只在业务成功时提交
	if cerr := tx.Commit(); cerr != nil {
		// 这里把 Commit 错误作为最终错误
		err = cerr
		return out, err
	}
	return out, nil
}
