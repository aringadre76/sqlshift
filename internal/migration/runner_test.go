package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbpkg "github.com/aringadre76/sqlshift/internal/db"
	"github.com/stretchr/testify/require"
)

func TestUp_HappyPath(t *testing.T) {
	runner, database, dialect := newTestRunner(t, fixtureDir("happy_path"))

	applied, err := runner.Up(context.Background())
	require.NoError(t, err)
	require.Len(t, applied, 3)

	history, historyErr := dialect.GetApplied(context.Background(), database, runner.TableName)
	require.NoError(t, historyErr)
	require.Len(t, history, 3)
}

func TestUp_Idempotency(t *testing.T) {
	runner, _, _ := newTestRunner(t, fixtureDir("happy_path"))

	first, err := runner.Up(context.Background())
	require.NoError(t, err)
	require.Len(t, first, 3)

	second, secondErr := runner.Up(context.Background())
	require.NoError(t, secondErr)
	require.Empty(t, second)
}

func TestUp_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "001_create_users.sql", validSQL("users"))
	writeMigration(t, dir, "002_bad.sql", "-- shift:up\nINVALID SQL;\n\n-- shift:down\nSELECT 1;")
	writeMigration(t, dir, "003_create_posts.sql", validSQL("posts"))

	runner, database, dialect := newTestRunner(t, dir)
	_, err := runner.Up(context.Background())
	require.Error(t, err)

	history, historyErr := dialect.GetApplied(context.Background(), database, runner.TableName)
	require.NoError(t, historyErr)
	require.Len(t, history, 1)
	require.Equal(t, 1, history[0].Version)
}

func TestUp_SequenceGapBlocked(t *testing.T) {
	runner, database, dialect := newTestRunner(t, fixtureDir("gap_in_sequence"))

	_, err := runner.Up(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "migration sequence gap")

	history, historyErr := dialect.GetApplied(context.Background(), database, runner.TableName)
	require.NoError(t, historyErr)
	require.Empty(t, history)
}

func TestUp_OutOfOrderBlocked(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "001_one.sql", validSQL("one"))
	writeMigration(t, dir, "002_two.sql", validSQL("two"))
	writeMigration(t, dir, "003_three.sql", validSQL("three"))
	writeMigration(t, dir, "004_four.sql", validSQL("four"))
	writeMigration(t, dir, "005_five.sql", validSQL("five"))

	runner, database, dialect := newTestRunner(t, dir)
	require.NoError(t, dialect.CreateHistoryTable(context.Background(), database, runner.TableName))
	require.NoError(t, insertHistory(database, runner.TableName, dbpkg.AppliedMigration{
		Version:         5,
		Name:            "five",
		Checksum:        mustLoadChecksum(t, filepath.Join(dir, "005_five.sql")),
		ExecutionTimeMs: 1,
	}))

	_, err := runner.Up(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "version is lower than already-applied migration 005")
}

func TestUp_FailsOnChecksumMismatch(t *testing.T) {
	runner, database, dialect := newTestRunner(t, fixtureDir("happy_path"))
	require.NoError(t, dialect.CreateHistoryTable(context.Background(), database, runner.TableName))
	require.NoError(t, insertHistory(database, runner.TableName, dbpkg.AppliedMigration{
		Version:         1,
		Name:            "create_users",
		Checksum:        "not-the-real-checksum",
		ExecutionTimeMs: 1,
	}))

	_, err := runner.Up(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "checksum mismatch for migration 1 (create_users)")

	history, historyErr := dialect.GetApplied(context.Background(), database, runner.TableName)
	require.NoError(t, historyErr)
	require.Len(t, history, 1)
}

func TestUp_FailsOnMissingAppliedFile(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "002_two.sql", validSQL("two"))

	runner, database, dialect := newTestRunner(t, dir)
	require.NoError(t, dialect.CreateHistoryTable(context.Background(), database, runner.TableName))
	require.NoError(t, insertHistory(database, runner.TableName, dbpkg.AppliedMigration{
		Version:         1,
		Name:            "missing",
		Checksum:        "whatever",
		ExecutionTimeMs: 1,
	}))

	_, err := runner.Up(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "recorded as applied but file no longer exists on disk")
}

func TestDown_HappyPath(t *testing.T) {
	runner, database, dialect := newTestRunner(t, fixtureDir("happy_path"))
	_, err := runner.Up(context.Background())
	require.NoError(t, err)

	reverted, downErr := runner.Down(context.Background())
	require.NoError(t, downErr)
	require.NotNil(t, reverted)
	require.Equal(t, 3, reverted.Version)

	history, historyErr := dialect.GetApplied(context.Background(), database, runner.TableName)
	require.NoError(t, historyErr)
	require.Len(t, history, 2)
}

func TestDown_NeverApplied(t *testing.T) {
	runner, _, _ := newTestRunner(t, fixtureDir("happy_path"))

	reverted, err := runner.Down(context.Background())
	require.NoError(t, err)
	require.Nil(t, reverted)
}

func TestDown_MissingDownSection(t *testing.T) {
	runner, _, _ := newTestRunner(t, fixtureDir("missing_down"))
	_, err := runner.Up(context.Background())
	require.NoError(t, err)

	_, downErr := runner.Down(context.Background())
	require.Error(t, downErr)
	require.Contains(t, downErr.Error(), "has no down section and cannot be reverted")
}

func TestStatus_MixedState(t *testing.T) {
	runner, database, dialect := newTestRunner(t, fixtureDir("happy_path"))
	require.NoError(t, dialect.CreateHistoryTable(context.Background(), database, runner.TableName))
	require.NoError(t, insertHistory(database, runner.TableName, dbpkg.AppliedMigration{
		Version:         1,
		Name:            "create_users",
		Checksum:        mustLoadChecksum(t, fixturePath("happy_path", "001_create_users.sql")),
		ExecutionTimeMs: 1,
	}))

	status, err := runner.Status(context.Background())
	require.NoError(t, err)
	require.Len(t, status, 3)
	require.Equal(t, StatusApplied, status[0].State)
	require.Equal(t, StatusPending, status[1].State)
}

func TestStatus_NoHistoryTable(t *testing.T) {
	runner, _, _ := newTestRunner(t, fixtureDir("happy_path"))

	status, err := runner.Status(context.Background())
	require.NoError(t, err)
	require.Len(t, status, 3)
	for _, entry := range status {
		require.Equal(t, StatusPending, entry.State)
	}
}

func TestValidate_ChecksumMismatch(t *testing.T) {
	runner, database, dialect := newTestRunner(t, fixtureDir("checksum_mismatch"))
	require.NoError(t, dialect.CreateHistoryTable(context.Background(), database, runner.TableName))
	require.NoError(t, insertHistory(database, runner.TableName, dbpkg.AppliedMigration{
		Version:         1,
		Name:            "create_users",
		Checksum:        "wrong",
		ExecutionTimeMs: 1,
	}))

	issues, err := runner.Validate(context.Background())
	require.NoError(t, err)
	require.True(t, hasIssue(issues, SeverityError, "checksum mismatch"))
}

func TestValidate_GapInSequence(t *testing.T) {
	runner, _, _ := newTestRunner(t, fixtureDir("gap_in_sequence"))

	issues, err := runner.Validate(context.Background())
	require.NoError(t, err)
	require.True(t, hasIssue(issues, SeverityError, "sequence gap"))
}

func TestValidate_MissingDown(t *testing.T) {
	runner, _, _ := newTestRunner(t, fixtureDir("missing_down"))

	issues, err := runner.Validate(context.Background())
	require.NoError(t, err)
	require.True(t, hasIssue(issues, SeverityWarning, "missing down section"))
	require.False(t, HasValidationErrors(issues))
}

func TestValidate_NoHistoryTable(t *testing.T) {
	runner, _, _ := newTestRunner(t, fixtureDir("gap_in_sequence"))

	issues, err := runner.Validate(context.Background())
	require.NoError(t, err)
	require.True(t, hasIssue(issues, SeverityError, "sequence gap"))
}

func newTestRunner(t *testing.T, migrationsDir string) (*Runner, *sql.DB, dbpkg.SQLiteDialect) {
	t.Helper()

	// database/sql pools connections; with bare :memory: each connection would
	// see a different in-memory database. Shared cache + one open connection
	// keeps the tests deterministic.
	database, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = database.Close()
	})
	database.SetMaxOpenConns(1)

	dialect := dbpkg.SQLiteDialect{}
	runner := NewRunner(database, dialect, migrationsDir, "shift_migrations")

	return runner, database, dialect
}

func writeMigration(t *testing.T, dir string, name string, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644))
}

func mustLoadChecksum(t *testing.T, path string) string {
	t.Helper()
	m, err := ParseFile(path)
	require.NoError(t, err)
	return m.Checksum
}

func insertHistory(database *sql.DB, tableName string, record dbpkg.AppliedMigration) error {
	_, err := database.Exec(
		"INSERT INTO "+tableName+" (version, name, checksum, execution_time_ms) VALUES (?, ?, ?, ?)",
		record.Version,
		record.Name,
		record.Checksum,
		record.ExecutionTimeMs,
	)
	return err
}

func hasIssue(issues []ValidationIssue, severity string, contains string) bool {
	for _, issue := range issues {
		if issue.Severity == severity && strings.Contains(issue.Message, contains) {
			return true
		}
	}
	return false
}
