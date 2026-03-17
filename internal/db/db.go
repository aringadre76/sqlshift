package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Dialect interface {
	CreateHistoryTable(ctx context.Context, db *sql.DB, tableName string) error
	InsertMigration(ctx context.Context, tx *sql.Tx, tableName string, record AppliedMigration) error
	DeleteMigration(ctx context.Context, tx *sql.Tx, tableName string, version int) error
	GetApplied(ctx context.Context, db *sql.DB, tableName string) ([]AppliedMigration, error)
	IsTableNotFound(err error) bool
}

type AppliedMigration struct {
	Version         int
	Name            string
	Checksum        string
	AppliedAt       string
	ExecutionTimeMs int
}

func Open(dsn string) (*sql.DB, Dialect, error) {
	driver, err := detectDriver(dsn)
	if err != nil {
		return nil, nil, err
	}

	switch driver {
	case "pgx":
		db, openErr := sql.Open("pgx", dsn)
		if openErr != nil {
			return nil, nil, fmt.Errorf("opening postgres connection: %w", openErr)
		}
		return db, PostgresDialect{}, nil
	case "mysql":
		formatted, formatErr := formatMySQLDSN(dsn)
		if formatErr != nil {
			return nil, nil, formatErr
		}
		db, openErr := sql.Open("mysql", formatted)
		if openErr != nil {
			return nil, nil, fmt.Errorf("opening mysql connection: %w", openErr)
		}
		return db, MySQLDialect{}, nil
	case "sqlite":
		sqliteDSN := strings.TrimPrefix(dsn, "sqlite://")
		db, openErr := sql.Open("sqlite", sqliteDSN)
		if openErr != nil {
			return nil, nil, fmt.Errorf("opening sqlite connection: %w", openErr)
		}
		return db, SQLiteDialect{}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported driver %q", driver)
	}
}

func detectDriver(dsn string) (string, error) {
	lower := strings.ToLower(dsn)
	switch {
	case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
		return "pgx", nil
	case strings.HasPrefix(lower, "mysql://"):
		return "mysql", nil
	case strings.HasPrefix(lower, "sqlite://"), strings.HasPrefix(lower, "file:"), dsn == ":memory:":
		return "sqlite", nil
	}

	switch filepath.Ext(lower) {
	case ".db", ".sqlite", ".sqlite3":
		return "sqlite", nil
	}

	return "", fmt.Errorf("unsupported database url %q", dsn)
}
