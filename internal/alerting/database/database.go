package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Database struct {
	db *sql.DB
}

func New(connString string) (*Database, error) {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *Database) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return d.db.ExecContext(ctx, q, args...)
}

// QueryContext exposes database/sql QueryContext for SELECT queries.
func (d *Database) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, q, args...)
}
