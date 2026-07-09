#!/usr/bin/env bash

set -euo pipefail

HASH_SERVICE_DIR="${HASH_SERVICE_DIR:-services/hash-service}"
PASSWORD_SERVICE_DIR="${PASSWORD_SERVICE_DIR:-services/password-service}"
WEB_DIR="${WEB_DIR:-apps/web}"

E2E_DATABASE_URL="${E2E_DATABASE_URL:-postgres://vaultforge:vaultforge_local_dev@127.0.0.1:5433/vaultforge_e2e?sslmode=disable}"
E2E_REDIS_URL="${E2E_REDIS_URL:-redis://127.0.0.1:6380/2}"
E2E_HASH_SERVICE_ADDR="${E2E_HASH_SERVICE_ADDR:-127.0.0.1:50052}"
E2E_PASSWORD_SERVICE_ADDR="${E2E_PASSWORD_SERVICE_ADDR:-127.0.0.1:50054}"

port_is_listening() {
	local addr="$1"
	local host="${addr%:*}"
	local port="${addr##*:}"

	(echo >"/dev/tcp/${host}/${port}") >/dev/null 2>&1
}

ensure_port_available() {
	local name="$1"
	local addr="$2"

	echo "Checking E2E ${name} service port ${addr}..."

	if port_is_listening "$addr"; then
		echo "${name} service port is already in use: ${addr}"
		echo "Stop the existing process or set the matching E2E service address to another port."
		exit 1
	fi
}

wait_for_service() {
	local name="$1"
	local addr="$2"
	local pid="$3"
	local attempts=0

	until port_is_listening "$addr"; do
		if ! kill -0 "$pid" >/dev/null 2>&1; then
			echo "${name} service exited before becoming ready"
			wait "$pid" || true
			exit 1
		fi

		attempts="$((attempts + 1))"

		if [[ "$attempts" -ge 30 ]]; then
			echo "${name} service did not become ready"
			exit 1
		fi

		sleep 1
	done

	if ! kill -0 "$pid" >/dev/null 2>&1; then
		echo "${name} service exited after readiness check"
		wait "$pid" || true
		exit 1
	fi

	echo "${name} service is ready."
}

cleanup() {
	if [[ -n "${hash_pid:-}" ]]; then
		kill "$hash_pid" >/dev/null 2>&1 || true
		wait "$hash_pid" >/dev/null 2>&1 || true
	fi

	if [[ -n "${password_pid:-}" ]]; then
		kill "$password_pid" >/dev/null 2>&1 || true
		wait "$password_pid" >/dev/null 2>&1 || true
	fi
}

trap cleanup EXIT

ensure_port_available "hash" "$E2E_HASH_SERVICE_ADDR"
ensure_port_available "password" "$E2E_PASSWORD_SERVICE_ADDR"

echo "Building hash service for E2E..."

(
	cd "$HASH_SERVICE_DIR"
	cargo build --quiet
)

echo "Building password service for E2E..."

(
	cd "$PASSWORD_SERVICE_DIR"
	cargo build --quiet
)

hash_service_binary="${HASH_SERVICE_DIR}/target/debug/hash-service"
password_service_binary="${PASSWORD_SERVICE_DIR}/target/debug/password-service"

echo "Starting hash service for E2E on ${E2E_HASH_SERVICE_ADDR}..."

HASH_SERVICE_BIND_ADDR="$E2E_HASH_SERVICE_ADDR" "$hash_service_binary" &
hash_pid="$!"

wait_for_service "hash" "$E2E_HASH_SERVICE_ADDR" "$hash_pid"

echo "Starting password service for E2E on ${E2E_PASSWORD_SERVICE_ADDR}..."

PASSWORD_SERVICE_BIND_ADDR="$E2E_PASSWORD_SERVICE_ADDR" "$password_service_binary" &
password_pid="$!"

wait_for_service "password" "$E2E_PASSWORD_SERVICE_ADDR" "$password_pid"

cd "$WEB_DIR"

E2E_DATABASE_URL="$E2E_DATABASE_URL" \
	E2E_REDIS_URL="$E2E_REDIS_URL" \
	E2E_HASH_SERVICE_ADDR="$E2E_HASH_SERVICE_ADDR" \
	E2E_PASSWORD_SERVICE_ADDR="$E2E_PASSWORD_SERVICE_ADDR" \
	npm run test:e2e
