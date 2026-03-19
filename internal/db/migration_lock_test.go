package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationAdvisoryLockKeyStable(t *testing.T) {
	k1 := migrationAdvisoryLockKey("shift_migrations")
	k2 := migrationAdvisoryLockKey("shift_migrations")
	require.Equal(t, k1, k2)
	require.NotEqual(t, k1, migrationAdvisoryLockKey("other_migrations"))
}

func TestMigrationNamedLockMySQLLength(t *testing.T) {
	name := migrationNamedLockMySQL("shift_migrations")
	require.LessOrEqual(t, len(name), 64)
	require.Contains(t, name, "sqlshift:")
}
