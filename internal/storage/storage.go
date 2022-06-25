package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type Storage struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Storage {
	return &Storage{
		db: db,
	}
}

type (
	transactionFunc func(tx *sqlx.Tx) error
	processFunc     func(msgs []Message) ([]int, error)
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

func (s *Storage) withTransaction(fn transactionFunc) error {
	tx, err := s.db.Beginx()
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

func (s *Storage) CreateSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS outbox_messages(
			id SERIAL PRIMARY KEY,
			queue_url TEXT NOT NULL,
			payload_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			dispatched BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),
			dispatched_at TIMESTAMP
	)`); err != nil {
		return err
	}

	return nil
}

func (s *Storage) Add(ctx context.Context, tx *sqlx.Tx, msg *Message) error {
	if _, err := tx.NamedExecContext(ctx, `
		INSERT INTO outbox_messages (
			queue_url,
			payload_type,
			payload
		)
		VALUES (
			:queue_url,
			:payload_type,
			:payload
	)`, msg); err != nil {
		return err
	}
	return nil
}

func (s *Storage) Process(ctx context.Context, limit int, fn processFunc) error {
	return s.withTransaction(func(tx *sqlx.Tx) error {
		messages, err := getMessages(ctx, tx, limit)
		if err != nil {
			return fmt.Errorf("get outbox messages error: %w", err)
		}

		ids, err := fn(messages)
		if err != nil {
			return fmt.Errorf("process messages error: %w", err)
		}

		if len(ids) == 0 {
			return nil
		}

		// or markAsDispatched
		if err := deleteMessages(ctx, tx, ids); err != nil {
			return fmt.Errorf("delete messages error: %w", err)
		}

		return nil
	})
}

func getMessages(ctx context.Context, tx *sqlx.Tx, limit int) ([]Message, error) {
	var messages []Message
	if err := tx.SelectContext(ctx, &messages, `
		SELECT id, queue_url, payload_type, payload
		FROM outbox_messages
		WHERE dispatched = FALSE
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit); err != nil {
		return nil, err
	}
	return messages, nil
}

func deleteMessages(ctx context.Context, tx *sqlx.Tx, ids []int) error {
	query, args, err := sqlx.In(`
		DELETE FROM outbox_messages
		WHERE id IN(?)
	`, ids)
	if err != nil {
		return fmt.Errorf("delete in statement: %w", err)
	}

	query = tx.Rebind(query)

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete exec: %w", err)
	}
	return nil
}

func markAsDispatched(ctx context.Context, tx *sqlx.Tx, ids []int) error {
	query, args, err := sqlx.In(`
		UPDATE outbox_messages
		SET dispatched = TRUE,
		dispatched_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
		WHERE id IN(?)
	`, ids)
	if err != nil {
		return fmt.Errorf("markAsDispatched in statement: %w", err)
	}

	query = tx.Rebind(query)

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("markAsDispatched exec: %w", err)
	}
	return nil
}
