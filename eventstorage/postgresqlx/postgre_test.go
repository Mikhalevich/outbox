package postgresqlx_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/Mikhalevich/outbox"
	"github.com/Mikhalevich/outbox/eventstorage/postgresqlx"
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

	if err := postgresqlx.New(gconn).CreateSchema(context.Background()); err != nil {
		log.Fatalf("unable to create schema: %v", err)
	}

	code := m.Run()

	if err := pool.Purge(resource); err != nil {
		log.Fatalf("could not purge resource: %s", err)
	}

	os.Exit(code)
}

//nolint:unused
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

func messageByQueueURL(url string) (*postgresqlx.Message, error) {
	var message postgresqlx.Message
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

func TestCreateSchema(t *testing.T) {
	t.Parallel()

	p := postgresqlx.New(gconn)
	err := p.CreateSchema(t.Context())
	require.NoError(t, err)
}

func TestAddSuccess(t *testing.T) {
	t.Parallel()

	event := outbox.Event{
		URL:         "test_queue_url_1",
		PayloadType: "test_payload_type",
		Payload:     []byte("test_payload"),
	}

	p := postgresqlx.New(gconn)
	err := postgresqlx.WithTransaction(gconn, func(tx *sqlx.Tx) error {
		err := p.Insert(t.Context(), tx, event)
		require.NoError(t, err)

		return nil
	})

	require.NoError(t, err)

	actualMsg, err := messageByQueueURL(event.URL)

	require.NoError(t, err)
	require.Equal(t, event.URL, actualMsg.QueueURL)
	require.Equal(t, event.PayloadType, actualMsg.PayloadType)
	require.Equal(t, event.Payload, actualMsg.Payload)
}

func TestAddTransactionError(t *testing.T) {
	t.Parallel()

	event := outbox.Event{
		URL:         "test_queue_url_2",
		PayloadType: "test_payload_type",
		Payload:     []byte("test_payload"),
	}

	p := postgresqlx.New(gconn)
	err := postgresqlx.WithTransaction(gconn, func(tx *sqlx.Tx) error {
		err := p.Insert(t.Context(), tx, event)
		require.NoError(t, err)

		return errors.New("some transaction error")
	})

	require.EqualError(t, err, "tx fn: some transaction error")

	_, err = messageByQueueURL(event.URL)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
