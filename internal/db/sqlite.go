package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type SQLiteDialect struct{}

func (SQLiteDialect) AcquireUpLock(context.Context, *sql.DB, string) (func(), error) {
	return func() {}, nil
}

func (SQLiteDialect) CreateHistoryTable(ctx context.Context, db *sql.DB, tableName string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning history table transaction: %w", err)
	}

	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	execution_time_ms INTEGER NOT NULL
)`, tableName)
	if _, err := tx.ExecContext(ctx, query); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("creating history table %s: %w", tableName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing history table transaction: %w", err)
	}

	return nil
}

func (SQLiteDialect) InsertMigration(ctx context.Context, tx *sql.Tx, tableName string, record AppliedMigration) error {
	query := fmt.Sprintf(
		"INSERT INTO %s (version, name, checksum, execution_time_ms) VALUES (?, ?, ?, ?)",
		tableName,
	)
	if _, err := tx.ExecContext(ctx, query, record.Version, record.Name, record.Checksum, record.ExecutionTimeMs); err != nil {
		return fmt.Errorf("inserting migration history for version %03d: %w", record.Version, err)
	}

	return nil
}

func (SQLiteDialect) DeleteMigration(ctx context.Context, tx *sql.Tx, tableName string, version int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE version = ?", tableName)
	if _, err := tx.ExecContext(ctx, query, version); err != nil {
		return fmt.Errorf("deleting migration history for version %03d: %w", version, err)
	}

	return nil
}

func (SQLiteDialect) GetApplied(ctx context.Context, db *sql.DB, tableName string) ([]AppliedMigration, error) {
	query := fmt.Sprintf(`
SELECT
	version,
	name,
	checksum,
	strftime('%%Y-%%m-%%dT%%H:%%M:%%SZ', applied_at) AS applied_at,
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

func (SQLiteDialect) IsTableNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such table")
}
