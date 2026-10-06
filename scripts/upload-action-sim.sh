#!/usr/bin/env bash
set -euo pipefail

: "${GIMME_URL:?GIMME_URL is required}"
: "${GIMME_TOKEN:?GIMME_TOKEN is required}"
: "${SIM_VERSION:?SIM_VERSION is required}"
sim_path="${SIM_PATH:-test/upload-action-sim}"
sim_name="${SIM_NAME:-upload-action-sim}"
sim_port="${SIM_PORT:-8765}"
goos="$(go env GOOS)"; goarch="$(go env GOARCH)"
tmp="$(mktemp -d)"
server_pid=""
trap 'if [[ -n "$server_pid" ]]; then kill "$server_pid" 2>/dev/null || true; fi; rm -rf "$tmp"' EXIT
mkdir -p "$tmp/latest/download" "$tmp/package"
go build -ldflags "-X main.version=sim" -o "$tmp/package/gimme-cli" ./cmd/gimme-cli
tar -czf "$tmp/latest/download/gimme-cli-$goos-$goarch.tar.gz" -C "$tmp/package" gimme-cli
python3 -m http.server "$sim_port" --bind 127.0.0.1 --directory "$tmp" >/dev/null 2>&1 & server_pid=$!; disown "$server_pid"
for _ in {1..50}; do
    if curl -fsS "http://127.0.0.1:$sim_port/" >/dev/null 2>&1; then break; fi
    sleep 0.1
done
GIMME_PATH="$sim_path" GIMME_NAME="$sim_name" GIMME_VERSION="$SIM_VERSION" GIMME_CLI_VERSION=latest GIMME_DOWNLOAD_BASE_URL="http://127.0.0.1:$sim_port" upload-action/run.sh
