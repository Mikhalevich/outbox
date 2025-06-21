package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type PostgresRepository struct {
	db *sqlx.DB
}

type TestData struct {
	ID        int    `db:"id"`
	IntVal    int    `db:"int_val"`
	StringVal string `db:"string_val"`
}

func NewPostgresRepository() (*PostgresRepository, error) {
	conn, err := sql.Open("postgres", "user=postgres password=123456 dbname=test host=dbpg port=5432 sslmode=disable")
	if err != nil {
		return nil, err
	}

	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(5)

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	pgDB := sqlx.NewDb(conn, "postgres")

	return &PostgresRepository{
		db: pgDB,
	}, nil
}

func (p *PostgresRepository) Close() error {
	return p.db.Close()
}

func (p *PostgresRepository) CreateSchema() error {
	if _, err := p.db.Exec(`
		CREATE TABLE IF NOT EXISTS Test(
			id SERIAL PRIMARY KEY,
			int_val integer NOT NULL,
			string_val varchar(100) NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema error: %w", err)
	}

	return nil
}

type TransactionFunc func(tx *sqlx.Tx) error

func (p *PostgresRepository) InsertTest(data *TestData, fn TransactionFunc) error {
	queryTemplate := fmt.Sprintf("INSERT INTO Test(int_val, string_val) %s", "VALUES(:int_val, :string_val)")

	tx, err := p.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin transaction error: %w", err)
	}

	if _, err := tx.NamedExec(queryTemplate, data); err != nil {
		return fmt.Errorf("named exec error: %w", err)
	}

	fn(tx)

	return tx.Commit()
}

func (p *PostgresRepository) GetTestData(ctx context.Context) ([]*TestData, error) {
	var tests []*TestData
	if err := p.db.SelectContext(ctx, &tests, "SELECT * FROM Test"); err != nil {
		return nil, fmt.Errorf("select context error: %w", err)
	}

	return tests, nil
}
