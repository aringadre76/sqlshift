#!/usr/bin/env bash
# End-to-end test: what a real user does — init, create, edit migration, up, status, validate, up again, down, status.
# SQLite runs by default (no Docker). Use --with-docker to start docker-compose.real-db.yml and exercise Postgres + MySQL too.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BINARY="./bin/sqlshift"
COMPOSE_FILE="$ROOT/docker-compose.real-db.yml"
WITH_DOCKER=false
ONLY=""
FALLBACK_PG_NAME="sqlshift-e2e-postgres"
FALLBACK_MY_NAME="sqlshift-e2e-mysql"

usage() {
  echo "Usage: $0 [options]"
  echo ""
  echo "Runs a fresh-project workflow per database (like README quickstart + real edits)."
  echo ""
  echo "Options:"
  echo "  --with-docker     docker compose up -d --wait, then SQLite + Postgres + MySQL"
  echo "  --only TARGET     sqlite | postgres | mysql (default: sqlite when not using --with-docker)"
  echo "  -h, --help"
}

die_usage() { usage; exit 2; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-docker) WITH_DOCKER=true; shift ;;
    --only)
      ONLY="${2:-}"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown: $1" >&2; die_usage ;;
  esac
done

if [[ ! -f "$BINARY" ]]; then
  echo "e2e: building $BINARY"
  make build
fi

# 0 = ready (--wait or immediate), 1 = started but may need sleep, 2 = hard failure
compose_up() {
  if docker compose version >/dev/null 2>&1; then
    if docker compose -f "$COMPOSE_FILE" up -d --wait 2>/dev/null; then
      return 0
    fi
    if docker compose -f "$COMPOSE_FILE" up -d; then
      return 1
    fi
    return 2
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    if docker-compose -f "$COMPOSE_FILE" up -d; then
      return 1
    fi
    return 2
  fi
  echo "error: need 'docker compose' (v2) or docker-compose" >&2
  return 2
}

docker_run_stack() {
  if ! docker inspect "$FALLBACK_PG_NAME" >/dev/null 2>&1; then
    if ! docker run -d \
      --name "$FALLBACK_PG_NAME" \
      -e POSTGRES_USER=sqlshift \
      -e POSTGRES_PASSWORD=sqlshift \
      -e POSTGRES_DB=sqlshift_real \
      -p 5432:5432 \
      postgres:16 >/dev/null; then
      echo "e2e: postgres fallback container not started (port may already be in use); reusing existing localhost:5432 service if available"
    fi
  else
    if ! docker start "$FALLBACK_PG_NAME" >/dev/null 2>&1; then
      echo "e2e: postgres fallback container exists but was not started; reusing existing localhost:5432 service if available"
    fi
  fi

  if ! docker inspect "$FALLBACK_MY_NAME" >/dev/null 2>&1; then
    if ! docker run -d \
      --name "$FALLBACK_MY_NAME" \
      -e MYSQL_ROOT_PASSWORD=rootpass \
      -e MYSQL_DATABASE=sqlshift_real \
      -e MYSQL_USER=sqlshift \
      -e MYSQL_PASSWORD=sqlshift \
      -p 3306:3306 \
      mysql:8 >/dev/null; then
      echo "e2e: mysql fallback container not started (port may already be in use); reusing existing localhost:3306 service if available"
    fi
  else
    if ! docker start "$FALLBACK_MY_NAME" >/dev/null 2>&1; then
      echo "e2e: mysql fallback container exists but was not started; reusing existing localhost:3306 service if available"
    fi
  fi
}

wait_fallback_db_ready() {
  local has_pg=0
  local has_my=0
  if docker inspect "$FALLBACK_PG_NAME" >/dev/null 2>&1; then
    [[ "$(docker inspect -f '{{.State.Running}}' "$FALLBACK_PG_NAME" 2>/dev/null || echo false)" == "true" ]] && has_pg=1
  fi
  if docker inspect "$FALLBACK_MY_NAME" >/dev/null 2>&1; then
    [[ "$(docker inspect -f '{{.State.Running}}' "$FALLBACK_MY_NAME" 2>/dev/null || echo false)" == "true" ]] && has_my=1
  fi
  if [[ "$has_pg" -eq 0 && "$has_my" -eq 0 ]]; then
    # We likely reused already-running services bound to host ports.
    sleep 3
    return 0
  fi
  local i=0
  for i in $(seq 1 90); do
    local pg_ok=0
    local my_ok=0
    if [[ "$has_pg" -eq 1 ]]; then
      docker exec "$FALLBACK_PG_NAME" pg_isready -U sqlshift -d sqlshift_real >/dev/null 2>&1 && pg_ok=1
    else
      pg_ok=1
    fi
    if [[ "$has_my" -eq 1 ]]; then
      docker exec "$FALLBACK_MY_NAME" mysqladmin ping -h 127.0.0.1 -usqlshift -psqlshift >/dev/null 2>&1 && my_ok=1
    else
      my_ok=1
    fi
    if [[ "$pg_ok" -eq 1 && "$my_ok" -eq 1 ]]; then
      return 0
    fi
    sleep 2
  done
  echo "error: docker-run fallback databases did not become ready in time" >&2
  return 1
}

wait_for_compose() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "error: docker not found" >&2
    exit 1
  fi
  echo "e2e: starting fake databases..."
  local st=0
  compose_up || st=$?
  if [[ "$st" -eq 2 ]]; then
    echo "e2e: compose unavailable; falling back to docker run containers"
    docker_run_stack
    wait_fallback_db_ready
    return
  fi
  if [[ "$st" -ne 0 ]]; then
    echo "e2e: waiting for databases (no --wait or docker-compose v1)..."
    sleep 25
  fi
}

# One isolated "user project": init → create → user edits SQL → full CLI cycle.
# Args: database_url  label  [extra sqlshift flags e.g. --table-name x]
run_journey() {
  local dburl=$1
  local label=$2
  shift 2
  local extra=( "$@" )

  local work
  work=$(mktemp -d)

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  User journey: $label"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  local rc=0
  (
    set -e
    cd "$work"

    run() { echo "+ $*"; "$@"; }

    run "$ROOT/$BINARY" init
    [[ -f .shift.toml ]] || { echo "error: .shift.toml missing" >&2; exit 1; }
    [[ -d migrations ]] || { echo "error: migrations/ missing" >&2; exit 1; }

    run "$ROOT/$BINARY" create add_user_widgets
    # `create` may print the path on stderr (Cobra); resolve file like a user would in their editor.
    shopt -s nullglob
    local paths=(migrations/*.sql)
    shopt -u nullglob
    [[ ${#paths[@]} -eq 1 ]] || { echo "error: expected exactly one migration after create, got ${#paths[@]}" >&2; exit 1; }
    local mig_path=${paths[0]}

    # Simulate a user filling in the template from `sqlshift create`.
    cat > "$mig_path" <<'EOSQL'
-- shift:up
CREATE TABLE user_widgets (
  id INTEGER PRIMARY KEY,
  label TEXT NOT NULL
);
INSERT INTO user_widgets (id, label) VALUES (1, 'e2e-user-journey');

-- shift:down
DROP TABLE user_widgets;
EOSQL

    run "$ROOT/$BINARY" up --database-url "$dburl" "${extra[@]}"
    run "$ROOT/$BINARY" status --database-url "$dburl" "${extra[@]}"
    run "$ROOT/$BINARY" validate --database-url "$dburl" "${extra[@]}"
    echo "+ second up (idempotent)"
    run "$ROOT/$BINARY" up --database-url "$dburl" "${extra[@]}"
    run "$ROOT/$BINARY" down --database-url "$dburl" "${extra[@]}"
    run "$ROOT/$BINARY" status --database-url "$dburl" "${extra[@]}"
  ) || rc=$?

  rm -rf "$work"
  [[ "$rc" -eq 0 ]] || exit "$rc"
  echo "e2e OK: $label"
}

if [[ "$WITH_DOCKER" == true ]]; then
  wait_for_compose
fi

uniq_table() {
  printf 'shift_e2e_%s_%s' "$(date +%s)" "$RANDOM"
}

if [[ -n "$ONLY" ]]; then
  case "$ONLY" in
    sqlite)
      w="$(mktemp -d)/app.db"
      run_journey "sqlite://$w" "SQLite (file)"
      ;;
    postgres)
      if [[ "$WITH_DOCKER" != true ]]; then
        echo "hint: for Postgres use --with-docker (compose) or start your own server on 127.0.0.1:5432" >&2
      fi
      [[ "$WITH_DOCKER" == true ]] && wait_for_compose
      t="$(uniq_table)"
      run_journey "postgres://sqlshift:sqlshift@127.0.0.1:5432/sqlshift_real?sslmode=disable" "Postgres" --table-name "$t"
      ;;
    mysql)
      if [[ "$WITH_DOCKER" != true ]]; then
        echo "hint: for MySQL use --with-docker (compose) or start your own server on 127.0.0.1:3306" >&2
      fi
      [[ "$WITH_DOCKER" == true ]] && wait_for_compose
      t="$(uniq_table)"
      run_journey "mysql://sqlshift:sqlshift@127.0.0.1:3306/sqlshift_real" "MySQL" --table-name "$t"
      ;;
    *)
      echo "error: --only must be sqlite, postgres, or mysql" >&2
      die_usage
      ;;
  esac
  exit 0
fi

# Default: SQLite only (accurate user flow, no Docker).
w="$(mktemp -d)/app.db"
run_journey "sqlite://$w" "SQLite (file)"

if [[ "$WITH_DOCKER" == true ]]; then
  t1="$(uniq_table)"
  run_journey "postgres://sqlshift:sqlshift@127.0.0.1:5432/sqlshift_real?sslmode=disable" "Postgres (compose)" --table-name "$t1"
  t2="$(uniq_table)"
  run_journey "mysql://sqlshift:sqlshift@127.0.0.1:3306/sqlshift_real" "MySQL (compose)" --table-name "$t2"
fi

echo ""
echo "All requested user journeys completed successfully."
