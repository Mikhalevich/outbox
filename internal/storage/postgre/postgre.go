package postgre

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/Mikhalevich/outbox/internal/storage"
)

type postgre struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *postgre {
	return &postgre{
		db: db,
	}
}

func (s *postgre) CreateSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS outbox_messages(
			id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			queue_url TEXT NOT NULL,
			payload_type TEXT NOT NULL,
			payload BYTEA NOT NULL,
			dispatched BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),
			dispatched_at TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("exec context: %w", err)
	}

	return nil
}

func (s *postgre) Add(ctx context.Context, tx *sqlx.Tx, msg *storage.Message) error {
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
		return fmt.Errorf("names exec: %w", err)
	}
	return nil
}

func (s *postgre) Process(ctx context.Context, limit int, fn storage.ProcessFunc) error {
	return storage.WithTransaction(s.db, func(tx *sqlx.Tx) error {
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

func getMessages(ctx context.Context, tx *sqlx.Tx, limit int) ([]storage.Message, error) {
	var messages []storage.Message
	if err := tx.SelectContext(ctx, &messages, `
		SELECT
			id,
			queue_url,
			payload_type,
			payload,
			dispatched,
			created_at,
			dispatched_at
		FROM outbox_messages
		WHERE dispatched = FALSE
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit); err != nil {
		return nil, fmt.Errorf("select context: %w", err)
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

//nolint
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
