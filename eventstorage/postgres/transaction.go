package postgres

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type TrxFn func(trx *sqlx.Tx) error

func WithTransaction(db *sqlx.DB, txFn TrxFn) error {
	trx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	//nolint:errcheck
	defer trx.Rollback()

	if err = txFn(trx); err != nil {
		//nolint:errcheck
		trx.Rollback()

		return fmt.Errorf("tx fn: %w", err)
	}

	if err := trx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
