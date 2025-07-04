package postgresqlx

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/Mikhalevich/outbox"
)

var _ outbox.EventStorage[*sqlx.Tx] = (*PostgreSqlx)(nil)

// PostgreSqlx postgres + sqlx implementation of outbox EventStorage interface.
type PostgreSqlx struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *PostgreSqlx {
	return &PostgreSqlx{
		db: db,
	}
}

func (s *PostgreSqlx) CreateSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS outbox_messages(
			id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			queue_url TEXT NOT NULL,
			payload_type TEXT NOT NULL,
			payload BYTEA NOT NULL,
			dispatched BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT (CURRENT_TIMESTAMP),
			dispatched_at TIMESTAMPTZ
	)`); err != nil {
		return fmt.Errorf("exec context: %w", err)
	}

	return nil
}

func (s *PostgreSqlx) Insert(ctx context.Context, trx *sqlx.Tx, event outbox.Event) error {
	if _, err := trx.NamedExecContext(ctx, `
		INSERT INTO outbox_messages (
			queue_url,
			payload_type,
			payload
		)
		VALUES (
			:queue_url,
			:payload_type,
			:payload
	)`, ConvertToMessage(event)); err != nil {
		return fmt.Errorf("named exec: %w", err)
	}

	return nil
}

func (s *PostgreSqlx) Process(ctx context.Context, limit int, processFn outbox.EventProcessorFn) error {
	if err := WithTransaction(s.db, func(trx *sqlx.Tx) error {
		messages, err := getMessages(ctx, trx, limit)
		if err != nil {
			return fmt.Errorf("get outbox messages: %w", err)
		}

		events := ConvertToEvents(messages)

		ids, err := processFn(events)
		if err != nil {
			return fmt.Errorf("process fn: %w", err)
		}

		if len(ids) == 0 {
			return nil
		}

		// or markAsDispatched
		if err := deleteMessages(ctx, trx, ids); err != nil {
			return fmt.Errorf("delete messages error: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("storage transaction: %w", err)
	}

	return nil
}

func getMessages(ctx context.Context, trx *sqlx.Tx, limit int) ([]Message, error) {
	var messages []Message
	if err := trx.SelectContext(ctx, &messages, `
		SELECT
			id,
			queue_url,
			payload_type,
			payload,
			dispatched,
			created_at,
			dispatched_at
		FROM
			outbox_messages
		WHERE
			dispatched = FALSE
		ORDER BY
			id
		LIMIT
			$1
		FOR UPDATE SKIP LOCKED
	`, limit); err != nil {
		return nil, fmt.Errorf("select context: %w", err)
	}

	return messages, nil
}

func deleteMessages(ctx context.Context, trx *sqlx.Tx, ids []outbox.EventID) error {
	query, args, err := sqlx.In(`
		DELETE FROM outbox_messages
		WHERE id IN(?)
	`, ids)
	if err != nil {
		return fmt.Errorf("delete in statement: %w", err)
	}

	query = trx.Rebind(query)

	if _, err := trx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete exec: %w", err)
	}

	return nil
}

//nolint:unused
func markAsDispatched(ctx context.Context, trx *sqlx.Tx, ids []outbox.EventID) error {
	query, args, err := sqlx.In(`
		UPDATE outbox_messages
		SET dispatched = TRUE,
		dispatched_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
		WHERE id IN(?)
	`, ids)
	if err != nil {
		return fmt.Errorf("markAsDispatched in statement: %w", err)
	}

	query = trx.Rebind(query)

	if _, err := trx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("markAsDispatched exec: %w", err)
	}

	return nil
}
