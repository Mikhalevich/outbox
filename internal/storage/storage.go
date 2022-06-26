package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type (
	TransactionFunc func(tx *sqlx.Tx) error
	ProcessFunc     func(msgs []Message) ([]int, error)
)

type Message struct {
	ID           int          `db:"id"`
	QueueURL     string       `db:"queue_url"`
	PayloadType  string       `db:"payload_type"`
	Payload      string       `db:"payload"`
	Dispatched   bool         `db:"dispatched"`
	CreatedAt    time.Time    `db:"created_at"`
	DispatchedAt sql.NullTime `db:"dispatched_at"`
}

func WithTransaction(db *sqlx.DB, fn TransactionFunc) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx error: %w", err)
	}

	if err = fn(tx); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("func tx error: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx error: %w", err)
	}

	return nil
}
