package db

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// migrationLockTimeoutSeconds is the maximum time to wait for MySQL GET_LOCK.
const migrationLockTimeoutSeconds = 30

// migrationAdvisoryLockKey returns a stable bigint for PostgreSQL pg_advisory_lock.
func migrationAdvisoryLockKey(tableName string) int64 {
	sum := sha256.Sum256([]byte("sqlshift:migrate:pg:" + tableName))
	// Use signed int64 range (PostgreSQL advisory locks use bigint).
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

// migrationNamedLockMySQL returns a lock name <= 64 bytes for GET_LOCK.
func migrationNamedLockMySQL(tableName string) string {
	sum := sha256.Sum256([]byte("sqlshift:migrate:mysql:" + tableName))
	return fmt.Sprintf("sqlshift:%x", sum[:10])
}
