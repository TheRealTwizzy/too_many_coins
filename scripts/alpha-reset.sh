#!/usr/bin/env bash
set -euo pipefail

if [[ "${PHASE:-}" != "alpha" ]]; then
  echo "Refusing to reset: PHASE must be 'alpha'." >&2
  exit 1
fi

if [[ "${ALPHA_RESET_CONFIRM:-}" != "I_UNDERSTAND" ]]; then
  echo "Refusing to reset: set ALPHA_RESET_CONFIRM=I_UNDERSTAND to proceed." >&2
  exit 1
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is not set." >&2
  exit 1
fi

echo "ALPHA RESET: dropping Phase-0 tables and re-applying schema_phase0.sql..."
psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -c "\
  DROP TABLE IF EXISTS admin_bootstrap_gate CASCADE; \
  DROP TABLE IF EXISTS player_state CASCADE; \
  DROP TABLE IF EXISTS sessions CASCADE; \
  DROP TABLE IF EXISTS players CASCADE; \
  DROP TABLE IF EXISTS accounts CASCADE;"
psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "$(dirname "$0")/../schema_phase0.sql"

echo "ALPHA RESET COMPLETE."
