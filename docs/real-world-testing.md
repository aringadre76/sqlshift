# Real-world and staging testing

Automated tests in this repo (`make test`, `make test-integration`) catch regressions in the Go code and dialect behavior. **They are not a substitute** for validating the same **binary**, **config**, and **database shape** you use in staging or production.

This document describes how to align with real deployments and how to run the **repeatable smoke harness** shipped in `scripts/smoke-real-db.sh`.

## How this maps to repo commands

| What | When to use |
|------|----------------|
| `make test` | Every change; fast unit + SQLite-backed runner tests. |
| `make test-integration` | When Docker is available; Postgres + MySQL via testcontainers. |
| `docker compose -f docker-compose.real-db.yml up -d` | Local “prod-shaped” Postgres 16 + MySQL 8. |
| `scripts/smoke-real-db.sh` | Same workflow as many deploy scripts: **build binary → up → status → validate → (down)**. |

## Environment parity

Match as much of production as practical:

- **Engine and version** — Use the same major version (e.g. Postgres 15 vs 16, MySQL 8.0 vs 8.4) in staging as in prod when possible.
- **Connection string** — Same query parameters you rely on in prod, e.g. Postgres `sslmode` (or TLS to a pooler), MySQL timeouts, etc.
- **Poolers** — If prod uses PgBouncer or similar, test at least once **through** the pooler in staging; some settings affect prepared statements or session features.
- **MySQL `sql_mode`** — Staging should match prod so the same SQL is accepted or rejected consistently.
- **SQLite** — Prefer a **file DSN** (`sqlite://./app.db`) for realistic locking and persistence; `:memory:` is fine for quick checks only.

## Identity and permissions

Avoid testing only as a superuser locally and then deploying as a restricted role.

A typical migration user needs to:

- **Create and alter** application objects your migrations define (tables, indexes, etc.).
- **Create** the history table (or have it pre-created by infra) in the target schema/database.
- **Insert** and **delete** rows in the history table for `up` / `down`.

Exact grants depend on your platform. If `sqlshift up` fails with permission errors in staging, fix grants there before prod.

## Recommended workflow order (staging / deploy-shaped)

1. **Build the release artifact** — `make build` or your CI-built `./bin/sqlshift` (not `go run`).
2. **Configuration** — Prefer the same mechanism as prod: `.shift.toml` and/or `SHIFT_DATABASE_URL`, `SHIFT_MIGRATIONS_DIR`, `SHIFT_TABLE_NAME`.
3. **`sqlshift up`** — Apply pending migrations (often in the deploy pipeline before or with the app).
4. **Application smoke** — Health checks, critical reads/writes, background workers.
5. **`sqlshift status` / `sqlshift validate`** — Sanity-check history vs files (e.g. after hotfixes or manual DB edits).

**`sqlshift down`** is mainly for **staging** and local drills. Many teams never run `down` in production; they ship a forward migration instead.

## Drills checklist

Run these periodically in **staging** (or local compose) with the **same binary and config** you use in deploy:

- [ ] **Clean apply** — Empty DB (or fresh schema), `up`, then app smoke.
- [ ] **Idempotent `up`** — Second `up` exits cleanly and applies nothing new.
- [ ] **`validate`** — Reports OK when history and files match.
- [ ] **Failed migration** — Introduce a broken migration in a throwaway branch; confirm deploy fails safely and you know how to recover (restore, fix forward, etc.).
- [ ] **Concurrent `up`** — Two deploy jobs or processes hitting `up` at once; both should complete without corrupt history (sqlshift uses per-dialect locking for Postgres/MySQL).
- [ ] **Rollback drill** — `down` one version in staging after realistic data exists; confirm app expectations and backups.
- [ ] **Backups** — Know how you would restore if a migration goes wrong; no one-size-fits-all automation here, but the drill should be documented for your org.

## Smoke script (sqlshift-only)

From the repository root:

```bash
chmod +x scripts/smoke-real-db.sh   # once, if needed
./scripts/smoke-real-db.sh --database-url "postgres://user:pass@host:5432/db?sslmode=disable"
```

Environment variable (optional): `SHIFT_DATABASE_URL` is used if `--database-url` is omitted.

Flags:

- `--database-url` — DSN (required unless `SHIFT_DATABASE_URL` is set).
- `--migrations-dir` — Defaults to `scripts/smoke-migrations`.
- `--binary` — Path to `sqlshift` (default `./bin/sqlshift`).
- `--table-name` — History table name (default: tool default / config).
- `--skip-down` — Stop after a second idempotent `up` (useful if you must not revert the smoke migration on a shared DB).

The script runs: **`up` → `status` → `validate` → `up` (again) → `down` (unless skipped) → `status`**. It does not run `psql` or `mysql` clients; for row-level checks, use your own queries or app tests.

If a previous run left the smoke migration applied, run with `--skip-down` only after you understand history state, or use a **fresh database**.

## Local prod-shaped databases (Docker Compose)

```bash
docker compose -f docker-compose.real-db.yml up -d
```

Example URLs (passwords match compose file):

- Postgres: `postgres://sqlshift:sqlshift@127.0.0.1:5432/sqlshift_real?sslmode=disable`
- MySQL: `mysql://sqlshift:sqlshift@127.0.0.1:3306/sqlshift_real`
- SQLite file: `sqlite://./smoke.db` (no compose needed)

## CI and staging pipelines

GitHub Actions: optional workflow **Real-world smoke** (`.github/workflows/real-world-smoke.yml`) runs the smoke script against **Postgres** and **MySQL** service containers. Trigger: `workflow_dispatch` or pushes to `main` that touch relevant paths.

In your own staging pipeline, add a step after building the binary:

```bash
./scripts/smoke-real-db.sh --database-url "$STAGING_DATABASE_URL" --skip-down
```

Use `--skip-down` if the staging DB is long-lived and you do not want the harness to drop the smoke table. Prefer a **dedicated staging schema or database** for full up/down cycles.

## Secrets

Never commit real credentials. Pass DSNs via CI secrets or local environment variables only.
