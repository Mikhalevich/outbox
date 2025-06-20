package postgre_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/Mikhalevich/outbox/internal/storage"
	"github.com/Mikhalevich/outbox/internal/storage/postgre"
)

var (
	//nolint:gochecknoglobals
	gconn *sqlx.DB
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("could not connect to docker: %s", err)
	}

	resource, err := pool.Run("postgres", "14-alpine", []string{
		"POSTGRES_PASSWORD=123456",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=test",
		"listen_addresses = '*'",
	})
	if err != nil {
		log.Fatalf("could not start resource: %s", err)
	}

	var (
		resPort = "5432/tcp"
		host    = resource.GetBoundIP(resPort)
		port    = resource.GetPort(resPort)
	)

	if err := pool.Retry(func() error {
		conn, err := sqlx.Open(
			"postgres",
			fmt.Sprintf("user=postgres password=123456 dbname=test host=%s port=%s sslmode=disable", host, port),
		)
		if err != nil {
			return fmt.Errorf("sql open: %w", err)
		}

		gconn = conn

		return conn.Ping()
	}); err != nil {
		log.Fatalf("could not connect to database: %s", err)
	}

	if err := postgre.New(gconn).CreateSchema(context.Background()); err != nil {
		log.Fatalf("unable to create schema: %v", err)
	}

	code := m.Run()

	if err := pool.Purge(resource); err != nil {
		log.Fatalf("could not purge resource: %s", err)
	}

	os.Exit(code)
}

func cleanup() {
	queryes := [...]string{
		"DELETE FROM outbox_messages",
	}

	for _, q := range queryes {
		if _, err := gconn.Exec(q); err != nil {
			log.Fatalf("unable to exec cleanup query: %s: %v", q, err)
		}
	}
}

func messageByQueueURL(url string) (*storage.Message, error) {
	var message storage.Message
	if err := gconn.Get(&message, `
		SELECT
			id,
			queue_url,
			payload_type,
			payload,
			dispatched,
			created_at,
			dispatched_at
		FROM outbox_messages
		WHERE queue_url = $1
	`, url); err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	return &message, nil
}

func compareMessages(t *testing.T, actual, expected *storage.Message) {
	t.Helper()

	var (
		now          = time.Now().UTC()
		actualCopy   = *actual
		expectedCopy = *expected
	)

	actualCopy.ID = 0
	actualCopy.CreatedAt = time.Date(actualCopy.CreatedAt.Year(), actualCopy.CreatedAt.Month(),
		actual.CreatedAt.Day(), actualCopy.CreatedAt.Hour(),
		0, 0, 0, time.UTC,
	)

	expectedCopy.CreatedAt = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

	require.Equal(t, expectedCopy, actualCopy)
}

func TestCreateSchema(t *testing.T) {
	t.Parallel()

	defer cleanup()

	p := postgre.New(gconn)
	err := p.CreateSchema(t.Context())
	require.NoError(t, err)
}

func TestAddSuccess(t *testing.T) {
	t.Parallel()

	defer cleanup()

	msg := storage.Message{
		QueueURL:    "test_queue_url",
		PayloadType: "test_payload_type",
		Payload:     []byte("test_payload"),
	}

	p := postgre.New(gconn)
	err := storage.WithTransaction(gconn, func(tx *sqlx.Tx) error {
		err := p.Add(t.Context(), tx, &msg)
		require.NoError(t, err)

		return nil
	})

	require.NoError(t, err)

	actualMsg, err := messageByQueueURL(msg.QueueURL)

	require.NoError(t, err)
	compareMessages(t, actualMsg, &storage.Message{
		QueueURL:    msg.QueueURL,
		PayloadType: msg.PayloadType,
		Payload:     msg.Payload,
		Dispatched:  false,
	})
}

func TestAddTransactionError(t *testing.T) {
	t.Parallel()

	defer cleanup()

	msg := storage.Message{
		QueueURL:    "test_queue_url",
		PayloadType: "test_payload_type",
		Payload:     []byte("test_payload"),
	}

	p := postgre.New(gconn)
	err := storage.WithTransaction(gconn, func(tx *sqlx.Tx) error {
		err := p.Add(t.Context(), tx, &msg)
		require.NoError(t, err)

		return errors.New("some transaction error")
	})

	require.EqualError(t, err, "func tx error: some transaction error")

	_, err = messageByQueueURL(msg.QueueURL)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
