#!/usr/bin/env bash

set -euo pipefail

HASH_SERVICE_DIR="${HASH_SERVICE_DIR:-services/hash-service}"
WEB_DIR="${WEB_DIR:-apps/web}"

E2E_DATABASE_URL="${E2E_DATABASE_URL:-postgres://vaultforge:vaultforge_local_dev@127.0.0.1:5433/vaultforge_e2e?sslmode=disable}"
E2E_REDIS_URL="${E2E_REDIS_URL:-redis://127.0.0.1:6380/2}"
E2E_HASH_SERVICE_ADDR="${E2E_HASH_SERVICE_ADDR:-127.0.0.1:50052}"

cleanup() {
	if [[ -n "${hash_pid:-}" ]]; then
		kill "$hash_pid" >/dev/null 2>&1 || true
		wait "$hash_pid" >/dev/null 2>&1 || true
	fi
}

trap cleanup EXIT

echo "Checking E2E hash service port ${E2E_HASH_SERVICE_ADDR}..."

if ! E2E_HASH_SERVICE_ADDR="$E2E_HASH_SERVICE_ADDR" python3 - <<'PY'
import os
import socket
import sys

host, port = os.environ["E2E_HASH_SERVICE_ADDR"].rsplit(":", 1)

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
    try:
        sock.bind((host, int(port)))
    except OSError:
        sys.exit(1)
PY
then
	echo "hash service port is already in use: ${E2E_HASH_SERVICE_ADDR}"
	echo "Stop the existing process or set E2E_HASH_SERVICE_ADDR to another port."
	exit 1
fi

echo "Building hash service for E2E..."

(
	cd "$HASH_SERVICE_DIR"
	cargo build --quiet
)

hash_service_binary="${HASH_SERVICE_DIR}/target/debug/hash-service"

echo "Starting hash service for E2E on ${E2E_HASH_SERVICE_ADDR}..."

HASH_SERVICE_BIND_ADDR="$E2E_HASH_SERVICE_ADDR" "$hash_service_binary" &
hash_pid="$!"

attempts=0
until E2E_HASH_SERVICE_ADDR="$E2E_HASH_SERVICE_ADDR" python3 - <<'PY' >/dev/null 2>&1
import os
import socket

host, port = os.environ["E2E_HASH_SERVICE_ADDR"].rsplit(":", 1)
socket.create_connection((host, int(port)), 1).close()
PY
do
	if ! kill -0 "$hash_pid" >/dev/null 2>&1; then
		echo "hash service exited before becoming ready"
		wait "$hash_pid" || true
		exit 1
	fi

	attempts="$((attempts + 1))"

	if [[ "$attempts" -ge 30 ]]; then
		echo "hash service did not become ready"
		exit 1
	fi

	sleep 1
done

if ! kill -0 "$hash_pid" >/dev/null 2>&1; then
	echo "hash service exited after readiness check"
	wait "$hash_pid" || true
	exit 1
fi

echo "Hash service is ready."

cd "$WEB_DIR"

E2E_DATABASE_URL="$E2E_DATABASE_URL" \
E2E_REDIS_URL="$E2E_REDIS_URL" \
E2E_HASH_SERVICE_ADDR="$E2E_HASH_SERVICE_ADDR" \
npm run test:e2e
