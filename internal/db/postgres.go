package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresDialect struct{}

func (PostgresDialect) AcquireUpLock(ctx context.Context, db *sql.DB, tableName string) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("reserving connection for migration lock: %w", err)
	}
	key := migrationAdvisoryLockKey(tableName)
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("acquiring pg advisory lock: %w", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			defer conn.Close()
			_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, key)
		})
	}, nil
}

func (PostgresDialect) CreateHistoryTable(ctx context.Context, db *sql.DB, tableName string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning history table transaction: %w", err)
	}

	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	execution_time_ms INTEGER NOT NULL
)`, tableName)
	if _, err := tx.ExecContext(ctx, query); err != nil {
		_ = tx.Rollback()
		if isPostgresBenignConcurrentDDL(err) {
			return nil
		}
		return fmt.Errorf("creating history table %s: %w", tableName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing history table transaction: %w", err)
	}

	return nil
}

func (PostgresDialect) InsertMigration(ctx context.Context, tx *sql.Tx, tableName string, record AppliedMigration) error {
	query := fmt.Sprintf(
		"INSERT INTO %s (version, name, checksum, execution_time_ms) VALUES ($1, $2, $3, $4)",
		tableName,
	)
	if _, err := tx.ExecContext(ctx, query, record.Version, record.Name, record.Checksum, record.ExecutionTimeMs); err != nil {
		return fmt.Errorf("inserting migration history for version %03d: %w", record.Version, err)
	}

	return nil
}

func (PostgresDialect) DeleteMigration(ctx context.Context, tx *sql.Tx, tableName string, version int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE version = $1", tableName)
	if _, err := tx.ExecContext(ctx, query, version); err != nil {
		return fmt.Errorf("deleting migration history for version %03d: %w", version, err)
	}

	return nil
}

func (PostgresDialect) GetApplied(ctx context.Context, db *sql.DB, tableName string) ([]AppliedMigration, error) {
	query := fmt.Sprintf(`
SELECT
	version,
	name,
	checksum,
	TO_CHAR(applied_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS applied_at,
	execution_time_ms
FROM %s
ORDER BY version`, tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying history table %s: %w", tableName, err)
	}
	defer rows.Close()

	var applied []AppliedMigration
	for rows.Next() {
		var record AppliedMigration
		if err := rows.Scan(&record.Version, &record.Name, &record.Checksum, &record.AppliedAt, &record.ExecutionTimeMs); err != nil {
			return nil, fmt.Errorf("scanning history row: %w", err)
		}
		applied = append(applied, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating history rows: %w", err)
	}

	return applied, nil
}

func (PostgresDialect) UpdateMigrationChecksum(ctx context.Context, db *sql.DB, tableName string, version int, checksum string) error {
	query := fmt.Sprintf("UPDATE %s SET checksum = $1 WHERE version = $2", tableName)
	_, err := db.ExecContext(ctx, query, checksum, version)
	if err != nil {
		return fmt.Errorf("updating checksum for version %03d: %w", version, err)
	}
	return nil
}

func (PostgresDialect) IsTableNotFound(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}

func isPostgresBenignConcurrentDDL(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "42P07": // duplicate_table
		return true
	case "23505": // unique_violation (e.g. concurrent CREATE TABLE IF NOT EXISTS racing on pg_type)
		if strings.Contains(pgErr.ConstraintName, "pg_type") {
			return true
		}
		return strings.Contains(strings.ToLower(pgErr.Message), "pg_type")
	default:
		return false
	}
}
