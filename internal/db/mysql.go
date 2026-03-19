package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type MySQLDialect struct{}

func (MySQLDialect) AcquireUpLock(ctx context.Context, db *sql.DB, tableName string) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("reserving connection for migration lock: %w", err)
	}
	name := migrationNamedLockMySQL(tableName)
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, name, migrationLockTimeoutSeconds).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("acquiring mysql named lock: %w", err)
	}
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		if !got.Valid {
			return nil, fmt.Errorf("acquiring mysql named lock: unexpected NULL from GET_LOCK")
		}
		return nil, fmt.Errorf("acquiring mysql named lock: timeout after %ds", migrationLockTimeoutSeconds)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			defer conn.Close()
			_, _ = conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, name)
		})
	}, nil
}

func formatMySQLDSN(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("unsupported MySQL DSN format -- v1 supports mysql://user:pass@host:port/dbname only")
	}
	if parsed.Scheme != "mysql" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("unsupported MySQL DSN format -- v1 supports mysql://user:pass@host:port/dbname only")
	}
	if parsed.Hostname() == "" || parsed.Port() == "" {
		return "", fmt.Errorf("unsupported MySQL DSN format -- v1 supports mysql://user:pass@host:port/dbname only")
	}
	if parsed.User == nil {
		return "", fmt.Errorf("unsupported MySQL DSN format -- v1 supports mysql://user:pass@host:port/dbname only")
	}
	password, hasPassword := parsed.User.Password()
	if parsed.User.Username() == "" || !hasPassword {
		return "", fmt.Errorf("unsupported MySQL DSN format -- v1 supports mysql://user:pass@host:port/dbname only")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return "", fmt.Errorf("unsupported MySQL DSN format -- v1 supports mysql://user:pass@host:port/dbname only")
	}

	dbName := strings.TrimPrefix(parsed.Path, "/")
	if strings.Contains(dbName, "/") || dbName == "" {
		return "", fmt.Errorf("unsupported MySQL DSN format -- v1 supports mysql://user:pass@host:port/dbname only")
	}

	// Migration up/down sections are executed as a single Exec; the MySQL driver
	// rejects multiple statements unless multiStatements is enabled.
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?multiStatements=true",
		parsed.User.Username(), password, parsed.Hostname(), parsed.Port(), dbName,
	), nil
}

func (MySQLDialect) CreateHistoryTable(ctx context.Context, db *sql.DB, tableName string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning history table transaction: %w", err)
	}

	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	execution_time_ms INTEGER NOT NULL
)`, tableName)
	if _, err := tx.ExecContext(ctx, query); err != nil {
		_ = tx.Rollback()
		if isMySQLBenignConcurrentDDL(err) {
			return nil
		}
		return fmt.Errorf("creating history table %s: %w", tableName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing history table transaction: %w", err)
	}

	return nil
}

func (MySQLDialect) InsertMigration(ctx context.Context, tx *sql.Tx, tableName string, record AppliedMigration) error {
	query := fmt.Sprintf(
		"INSERT INTO %s (version, name, checksum, execution_time_ms) VALUES (?, ?, ?, ?)",
		tableName,
	)
	if _, err := tx.ExecContext(ctx, query, record.Version, record.Name, record.Checksum, record.ExecutionTimeMs); err != nil {
		return fmt.Errorf("inserting migration history for version %03d: %w", record.Version, err)
	}

	return nil
}

func (MySQLDialect) DeleteMigration(ctx context.Context, tx *sql.Tx, tableName string, version int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE version = ?", tableName)
	if _, err := tx.ExecContext(ctx, query, version); err != nil {
		return fmt.Errorf("deleting migration history for version %03d: %w", version, err)
	}

	return nil
}

func (MySQLDialect) GetApplied(ctx context.Context, db *sql.DB, tableName string) ([]AppliedMigration, error) {
	query := fmt.Sprintf(`
SELECT
	version,
	name,
	checksum,
	DATE_FORMAT(applied_at, '%%Y-%%m-%%dT%%H:%%i:%%sZ') AS applied_at,
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

func (MySQLDialect) IsTableNotFound(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1146
}

func isMySQLBenignConcurrentDDL(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1050
}
