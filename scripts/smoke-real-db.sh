#!/usr/bin/env bash
# Real-world smoke: build binary (if needed), then up → status → validate → up → down → status.
# Usage: see docs/real-world-testing.md
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BINARY="./bin/sqlshift"
MIGRATIONS_DIR="$ROOT/scripts/smoke-migrations"
TABLE_NAME_ARGS=()
SKIP_DOWN=false
DATABASE_URL="${SHIFT_DATABASE_URL:-}"

usage() {
  echo "Usage: $0 --database-url <url> [options]"
  echo "   or: SHIFT_DATABASE_URL=... $0 [options]"
  echo ""
  echo "Options:"
  echo "  --migrations-dir DIR   (default: scripts/smoke-migrations)"
  echo "  --binary PATH          (default: ./bin/sqlshift)"
  echo "  --table-name NAME      (optional history table)"
  echo "  --skip-down            do not run down or final status-after-down"
}

die_usage() {
  usage
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --database-url)
      DATABASE_URL="${2:-}"
      shift 2
      ;;
    --migrations-dir)
      MIGRATIONS_DIR="${2:-}"
      shift 2
      ;;
    --binary)
      BINARY="${2:-}"
      shift 2
      ;;
    --table-name)
      TABLE_NAME_ARGS=(--table-name "${2:-}")
      shift 2
      ;;
    --skip-down)
      SKIP_DOWN=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      die_usage
      ;;
  esac
done

if [[ -z "$DATABASE_URL" ]]; then
  echo "error: set --database-url or SHIFT_DATABASE_URL" >&2
  die_usage
fi

if [[ ! -f "$BINARY" ]]; then
  echo "smoke: $BINARY missing — running make build"
  make build
fi

if [[ ! -x "$BINARY" ]] && [[ -f "$BINARY" ]]; then
  chmod +x "$BINARY" 2>/dev/null || true
fi

run() {
  echo "+ $*"
  "$@"
}

common=(--database-url "$DATABASE_URL" --migrations-dir "$MIGRATIONS_DIR")
[[ ${#TABLE_NAME_ARGS[@]} -eq 0 ]] || common+=("${TABLE_NAME_ARGS[@]}")

echo "=== sqlshift smoke (repo: $ROOT) ==="
run "$BINARY" up "${common[@]}"
run "$BINARY" status "${common[@]}"
run "$BINARY" validate "${common[@]}"
echo "=== second up (idempotent) ==="
run "$BINARY" up "${common[@]}"

if [[ "$SKIP_DOWN" == true ]]; then
  echo "=== --skip-down: done ==="
  exit 0
fi

run "$BINARY" down "${common[@]}"
run "$BINARY" status "${common[@]}"
echo "=== smoke OK ==="
