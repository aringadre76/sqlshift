//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dbpkg "github.com/aringadre76/sqlshift/internal/db"
	"github.com/aringadre76/sqlshift/internal/migration"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresUpDownStatusCycle(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	ctx := context.Background()
	database, runner := newPostgresRunner(t, ctx, fixtureDir("happy_path"))

	applied, err := runner.Up(ctx)
	require.NoError(t, err)
	require.Len(t, applied, 3)

	status, statusErr := runner.Status(ctx)
	require.NoError(t, statusErr)
	require.Len(t, status, 3)
	require.Equal(t, migration.StatusApplied, status[0].State)

	reverted, downErr := runner.Down(ctx)
	require.NoError(t, downErr)
	require.NotNil(t, reverted)
	require.Equal(t, 3, reverted.Version)

	var appliedAt string
	rowErr := database.QueryRowContext(
		ctx,
		"SELECT TO_CHAR(applied_at AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') FROM shift_migrations ORDER BY version LIMIT 1",
	).Scan(&appliedAt)
	require.NoError(t, rowErr)
	require.NotEmpty(t, appliedAt)
}

func TestPostgresUpIdempotency(t *testing.T) {
	ctx := context.Background()
	_, runner := newPostgresRunner(t, ctx, fixtureDir("happy_path"))

	first, err := runner.Up(ctx)
	require.NoError(t, err)
	require.Len(t, first, 3)

	second, secondErr := runner.Up(ctx)
	require.NoError(t, secondErr)
	require.Empty(t, second)
}

func TestPostgresPartialFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "001_create_users.sql", validSQL("users"))
	writeMigration(t, dir, "002_bad.sql", "-- shift:up\nINVALID SQL;\n\n-- shift:down\nSELECT 1;")
	writeMigration(t, dir, "003_create_posts.sql", validSQL("posts"))

	database, runner := newPostgresRunner(t, ctx, dir)
	_, err := runner.Up(ctx)
	require.Error(t, err)

	var count int
	require.NoError(t, database.QueryRowContext(ctx, "SELECT COUNT(*) FROM shift_migrations").Scan(&count))
	require.Equal(t, 1, count)
}

func TestPostgresOutOfOrderBlocked(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "001_one.sql", validSQL("one"))
	writeMigration(t, dir, "002_two.sql", validSQL("two"))
	writeMigration(t, dir, "003_three.sql", validSQL("three"))
	writeMigration(t, dir, "004_four.sql", validSQL("four"))
	writeMigration(t, dir, "005_five.sql", validSQL("five"))

	database, runner := newPostgresRunner(t, ctx, dir)
	require.NoError(t, runner.Dialect.CreateHistoryTable(ctx, database, runner.TableName))
	checksum := mustChecksum(t, filepath.Join(dir, "005_five.sql"))
	_, err := database.ExecContext(
		ctx,
		"INSERT INTO shift_migrations (version, name, checksum, execution_time_ms) VALUES ($1, $2, $3, $4)",
		5, "five", checksum, 1,
	)
	require.NoError(t, err)

	_, upErr := runner.Up(ctx)
	require.Error(t, upErr)
	require.Contains(t, upErr.Error(), "already-applied migration 005")
}

func newPostgresRunner(t *testing.T, ctx context.Context, migrationsDir string) (*sql.DB, *migration.Runner) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "sqlshift",
			"POSTGRES_PASSWORD": "sqlshift",
			"POSTGRES_DB":       "sqlshift",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections"),
		),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://sqlshift:sqlshift@%s:%s/sqlshift?sslmode=disable", host, port.Port())
	database, dialect, err := dbpkg.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = database.Close()
	})

	return database, migration.NewRunner(database, dialect, migrationsDir, "shift_migrations")
}

func fixtureDir(name string) string {
	return filepath.Join("..", "testdata", "migrations", name)
}

func writeMigration(t *testing.T, dir string, name string, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644))
}

func validSQL(name string) string {
	return "-- shift:up\nCREATE TABLE " + name + " (id INTEGER PRIMARY KEY);\n\n-- shift:down\nDROP TABLE " + name + ";"
}

func mustChecksum(t *testing.T, path string) string {
	t.Helper()
	m, err := migration.ParseFile(path)
	require.NoError(t, err)
	return m.Checksum
}
