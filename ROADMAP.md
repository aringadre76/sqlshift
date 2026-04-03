# sqlshift Roadmap

## Current State

sqlshift is a SQL-first database migration CLI tool written in Go, supporting PostgreSQL, MySQL, and SQLite. It emphasizes explicit, human-written SQL files committed to git.

### Implemented Features

**Core Commands:**
- `init` - Creates migrations directory and config file
- `create <name>` - Creates numbered migration files
- `up` - Applies all pending migrations
- `down` - Reverts the last applied migration
- `status` - Shows migration status
- `validate` - Validates migration integrity

**Safety Features:**
- Checksum validation (SHA-256)
- Concurrent migration locking
- Out-of-order protection
- Sequence gap detection
- Transaction safety

**Testing:**
- Unit tests with SQLite
- Integration tests with testcontainers (Postgres/MySQL)
- E2E user journey tests
- Real-world smoke tests

## Priority Features (Next Steps)

### 1. Targeted Migrations (HIGH)
**Feature:** `up --to <version>` and `down --to <version>`
- Apply or rollback to a specific version
- Most requested feature in migration tools
- Builds on existing planning logic
- Impact: HIGH | Safety: MEDIUM

### 2. Dry-Run Mode (HIGH)
**Feature:** `--dry-run` flag for `up` and `down`
- Preview migrations without executing
- Essential for CI/CD safety
- Read-only operation
- Impact: HIGH | Safety: HIGH

### 3. Verbose Output (MEDIUM)
**Feature:** `-v` / `--verbose` flag
- Show actual SQL statements during execution
- Critical for debugging
- Simple implementation
- Impact: MEDIUM | Safety: HIGH

### 4. Repair Command (MEDIUM)
**Feature:** `repair`
- Fix checksum mismatches
- Mark migrations as resolved
- Currently requires manual DB edits
- Impact: MEDIUM | Safety: MEDIUM

### 5. Baseline Command (MEDIUM)
**Feature:** `baseline <version>`
- Mark existing database at specific version
- Enable adoption on brownfield projects
- No migration path currently exists
- Impact: MEDIUM | Safety: MEDIUM

### 6. Clean Command (LOW)
**Feature:** `clean` (with `--force`)
- Drop all database objects
- Useful for dev/testing
- Requires confirmation
- Impact: LOW | Safety: LOW

### 7. JSON Output (MEDIUM)
**Feature:** `--output json`
- Machine-readable output
- CI/CD integration
- Structured logging
- Impact: MEDIUM | Safety: HIGH

## Technical Debt

### Known Issues:
1. MySQL DSN parsing limited to basic URL format
2. No context cancellation in long-running migrations
3. Error messages could be more actionable
4. Migration locking only for `up`, not `down`
5. Table name validation pattern-only (SQL injection risk)

### Test Coverage Gaps:
- No tests for `init` command
- No tests for `create` command
- No tests for config loading edge cases
- No tests for MySQL DSN format errors
- No tests for SQLite file-based databases

## Recommendation

**Start with Dry-Run Mode (#2)** because:
- Simple to implement
- Very high safety value
- No database schema changes
- Perfect for CI/CD workflows
- Foundation for other features
