//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	dbpkg "github.com/aringadre76/sqlshift/internal/db"
	"github.com/aringadre76/sqlshift/internal/migration"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMySQLUpDownStatusCycle(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	ctx := context.Background()
	_, runner := newMySQLRunner(t, ctx, fixtureDir("happy_path"))

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
}

func TestMySQLUpIdempotency(t *testing.T) {
	ctx := context.Background()
	_, runner := newMySQLRunner(t, ctx, fixtureDir("happy_path"))

	first, err := runner.Up(ctx)
	require.NoError(t, err)
	require.Len(t, first, 3)

	second, secondErr := runner.Up(ctx)
	require.NoError(t, secondErr)
	require.Empty(t, second)
}

func TestMySQLPartialFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "001_create_users.sql", validSQL("users"))
	writeMigration(t, dir, "002_bad.sql", "-- shift:up\nINVALID SQL;\n\n-- shift:down\nSELECT 1;")
	writeMigration(t, dir, "003_create_posts.sql", validSQL("posts"))

	database, runner := newMySQLRunner(t, ctx, dir)
	_, err := runner.Up(ctx)
	require.Error(t, err)

	var count int
	require.NoError(t, database.QueryRowContext(ctx, "SELECT COUNT(*) FROM shift_migrations").Scan(&count))
	require.Equal(t, 1, count)
}

func TestMySQLOutOfOrderBlocked(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "001_one.sql", validSQL("one"))
	writeMigration(t, dir, "002_two.sql", validSQL("two"))
	writeMigration(t, dir, "003_three.sql", validSQL("three"))
	writeMigration(t, dir, "004_four.sql", validSQL("four"))
	writeMigration(t, dir, "005_five.sql", validSQL("five"))

	database, runner := newMySQLRunner(t, ctx, dir)
	require.NoError(t, runner.Dialect.CreateHistoryTable(ctx, database, runner.TableName))
	checksum := mustChecksum(t, filepath.Join(dir, "005_five.sql"))
	_, err := database.ExecContext(
		ctx,
		"INSERT INTO shift_migrations (version, name, checksum, execution_time_ms) VALUES (?, ?, ?, ?)",
		5, "five", checksum, 1,
	)
	require.NoError(t, err)

	_, upErr := runner.Up(ctx)
	require.Error(t, upErr)
	require.Contains(t, upErr.Error(), "already-applied migration 005")
}

func TestMySQLDDLImplicitCommitDocumentation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "001_create_then_fail.sql", "-- shift:up\nCREATE TABLE ddl_commit_test (id INTEGER PRIMARY KEY);\nINVALID SQL;\n\n-- shift:down\nDROP TABLE ddl_commit_test;")

	database, runner := newMySQLRunner(t, ctx, dir)
	_, err := runner.Up(ctx)
	require.Error(t, err)

	var count int
	query := `
SELECT COUNT(*)
FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_name = 'ddl_commit_test'
`
	require.NoError(t, database.QueryRowContext(ctx, query).Scan(&count))
	require.Equal(t, 1, count)
}

func newMySQLRunner(t *testing.T, ctx context.Context, migrationsDir string) (*sql.DB, *migration.Runner) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "mysql:8",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "rootpass",
			"MYSQL_DATABASE":      "sqlshift",
			"MYSQL_USER":          "sqlshift",
			"MYSQL_PASSWORD":      "sqlshift",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("3306/tcp"),
			wait.ForLog("ready for connections"),
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
	port, err := container.MappedPort(ctx, "3306/tcp")
	require.NoError(t, err)

	dsn := fmt.Sprintf("mysql://sqlshift:sqlshift@%s:%s/sqlshift", host, port.Port())
	database, dialect, err := dbpkg.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = database.Close()
	})

	return database, migration.NewRunner(database, dialect, migrationsDir, "shift_migrations")
}
