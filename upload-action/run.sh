#!/usr/bin/env bash
set -euo pipefail

fail() { echo "gimme upload action: $*" >&2; exit 1; }

runner_os="${RUNNER_OS:-}"
if [[ -z "$runner_os" ]]; then runner_os="$(uname -s)"; fi
case "$runner_os" in
  Linux|linux) goos=linux ;;
  macOS|MacOS|Darwin|darwin) goos=darwin ;;
  Windows|windows|Windows_*|MINGW*|MSYS*|CYGWIN*) fail "Windows runners are not supported; use gimme-cli directly" ;;
  *) fail "unsupported operating system: $runner_os" ;;
esac

runner_arch="${RUNNER_ARCH:-}"
if [[ -z "$runner_arch" ]]; then runner_arch="$(uname -m)"; fi
case "$runner_arch" in X64|x64|x86_64|amd64) goarch=amd64 ;; ARM64|arm64|aarch64) goarch=arm64 ;; X86|x86|i386|i686|386) goarch=386 ;; *) fail "unsupported architecture: $runner_arch" ;; esac
if [[ "$goos/$goarch" == "darwin/386" ]]; then fail "unsupported platform: $goos/$goarch"; fi

asset="gimme-cli-$goos-$goarch.tar.gz"
base="${GIMME_DOWNLOAD_BASE_URL:-https://github.com/ziggornif/gimme/releases}"
cli_version="${GIMME_CLI_VERSION:-latest}"
if [[ "$cli_version" == latest ]]; then url="$base/latest/download/$asset"; else url="$base/download/$cli_version/$asset"; fi
temp_root="${RUNNER_TEMP:-}"
if [[ -n "$temp_root" ]]; then temp_dir="$(mktemp -d "$temp_root/gimme-cli.XXXXXX")"; else temp_dir="$(mktemp -d)"; fi
trap 'rm -rf "$temp_dir"' EXIT
if ! curl -fsSL "$url" -o "$temp_dir/$asset"; then fail "failed to download $url"; fi
tar -xzf "$temp_dir/$asset" -C "$temp_dir"
chmod +x "$temp_dir/gimme-cli"
args=(push "$GIMME_PATH" --name "$GIMME_NAME")
if [[ -n "${GIMME_VERSION:-}" ]]; then args+=(--version "$GIMME_VERSION"); fi
"$temp_dir/gimme-cli" "${args[@]}"
