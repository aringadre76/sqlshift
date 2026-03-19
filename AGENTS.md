## Learned User Preferences
- Expects plans to be fully specified and corrected before any implementation, preferring an explicit review-and-fix loop on the plan document.
- Prefers test-driven development for new features and bug fixes (write tests first, then minimal code to pass).
- Wants SQL migration tools to be conservative and safety-first: `up` must refuse to run on checksum mismatches, sequence gaps, duplicate versions, or out-of-order migrations.
- Cares that public APIs and error contracts are explicit and consistent across the codebase (no conflicting interface definitions or underspecified behaviors).
- Dislikes surprising side effects in read-only commands (e.g., `status` or `validate` should not mutate the database).
- Is sensitive to naming and UX conflicts (e.g., avoiding CLI binary names that collide with shell builtins like `shift`).
- If something cannot run in the agent shell session, wants a direct handoff: provide copy-paste commands for the user to run and do not pursue workarounds unless the user also cannot run them.

## Learned Workspace Facts
- This repo is the `sqlshift` project: a Go-based SQL-first migration CLI that targets Postgres, MySQL, and SQLite.
- The module path is `github.com/aringadre76/sqlshift` and the built binary is named `sqlshift`.
- The CLI is implemented with Cobra and Viper, with configuration coming from `.shift.toml`, `SHIFT_*` env vars, and CLI flags in that priority order.
- Database access uses `pgx/v5` for Postgres, `go-sql-driver/mysql` for MySQL, and `modernc.org/sqlite` for SQLite so that builds stay CGO-free.
- Migration files live under `testdata/migrations/` for tests and use `-- shift:up` / `-- shift:down` markers plus zero-padded versions like `001_name.sql`.
- The project has a three-level test strategy: unit tests, fast SQLite-backed runner tests, and testcontainers-based integration tests for Postgres and MySQL.
