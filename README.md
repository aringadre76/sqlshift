# sqlshift

SQL-first database migrations in a single Go binary. No JVM, no XML, no YAML, no open-core split.

`sqlshift` is built for teams that want explicit, human-written SQL files committed to git and applied in order across PostgreSQL, MySQL, and SQLite.

## Quickstart

```bash
make build
./bin/sqlshift init
./bin/sqlshift create add_users_table
./bin/sqlshift up --database-url "sqlite://local.db"
./bin/sqlshift status --database-url "sqlite://local.db"
```

## Migration Format

Migration files live in `./migrations` by default and use numbered filenames:

```text
001_create_users.sql
002_add_name_column.sql
003_create_posts.sql
```

Each file contains `-- shift:up` and an optional `-- shift:down` section:

```sql
-- shift:up
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE
);

-- shift:down
DROP TABLE users;
```

If `-- shift:down` is omitted, the migration is treated as irreversible. `sqlshift validate` warns about that, and `sqlshift down` refuses to revert it.

## Commands

- `sqlshift init` creates `./migrations/` and `.shift.toml`
- `sqlshift create <name>` creates the next numbered migration file
- `sqlshift up` applies all pending migrations
- `sqlshift down` reverts the last applied migration
- `sqlshift status` shows applied and pending migrations
- `sqlshift validate` checks sequence gaps, checksum mismatches, duplicate versions, and missing down sections

## Configuration

Configuration can come from CLI flags, environment variables, or `.shift.toml`.

Priority order:

1. CLI flags
2. Environment variables
3. `.shift.toml`

Example config:

```toml
database_url = "postgres://user:pass@localhost:5432/mydb?sslmode=disable"
migrations_dir = "./migrations"
table_name = "shift_migrations"
```

Environment variables:

- `SHIFT_DATABASE_URL`
- `SHIFT_MIGRATIONS_DIR`
- `SHIFT_TABLE_NAME`

## Development

```bash
make test
make test-integration
make lint
make release-dry
```

## Comparison

| Tool | Style | Runtime | License |
|------|-------|---------|---------|
| `sqlshift` | Versioned SQL files | Single Go binary | MIT |
| Flyway Community | Versioned SQL files | JVM | Source-available product strategy |
| Liquibase | Changelog-driven | JVM | FSL in modern versions |
| Atlas | Declarative desired state | Go binary | OSS, different workflow |

## License

MIT. See [LICENSE](LICENSE).
