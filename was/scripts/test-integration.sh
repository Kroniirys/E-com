#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  docker compose --project-directory .. --env-file ../.env.example --profile test up -d --wait postgres-test
  export TEST_DATABASE_URL='postgres://ecom_test:isolated_test_only@127.0.0.1:55433/ecom_test?sslmode=disable'
fi
APP_ENV=test "${GO:-go}" test -tags=integration -count=1 -v ./tests/integration
