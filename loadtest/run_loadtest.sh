#!/usr/bin/env bash
# run_loadtest.sh — builds servers, runs load test, cleans up.
#
# Usage:
#   ./run_loadtest.sh [runner flags]
#
# Examples:
#   ./run_loadtest.sh
#   ./run_loadtest.sh -duration 20s -conns 200
#   ./run_loadtest.sh -servers zen,std -duration 15s -conns 50

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/.build"

mkdir -p "$BUILD_DIR"

# ── colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

log()  { echo -e "${BOLD}${CYAN}▶ $*${RESET}"; }
ok()   { echo -e "${GREEN}  ✓ $*${RESET}"; }
warn() { echo -e "${YELLOW}  ⚠ $*${RESET}"; }
fail() { echo -e "${RED}  ✗ $*${RESET}"; exit 1; }

# ── cleanup ───────────────────────────────────────────────────────────────────
PIDS=()
cleanup() {
  echo ""
  log "Stopping servers..."
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null && ok "stopped PID $pid" || true
  done
}
trap cleanup EXIT INT TERM

# ── prerequisites ─────────────────────────────────────────────────────────────
command -v go >/dev/null 2>&1 || fail "go not found in PATH"
GO_VER=$(go version | awk '{print $3}')
log "Using $GO_VER"

# ── deps ──────────────────────────────────────────────────────────────────────
log "Downloading dependencies..."
cd "$SCRIPT_DIR"
go mod tidy
ok "dependencies ready"

# ── build ─────────────────────────────────────────────────────────────────────
log "Building server binaries..."

build_server() {
  local name=$1
  local pkg=$2
  go build -o "$BUILD_DIR/$name" "$pkg" && ok "$name"
}

build_server zen_server    ./cmd/zen_server
build_server gin_server    ./cmd/gin_server
build_server echo_server   ./cmd/echo_server
build_server std_server    ./cmd/std_server
build_server runner        ./runner

# ── start servers ─────────────────────────────────────────────────────────────
log "Starting servers..."

start_server() {
  local name=$1
  local bin=$2
  local port=$3
  local log_file="$BUILD_DIR/${name}.log"

  "$BUILD_DIR/$bin" > "$log_file" 2>&1 &
  local pid=$!
  PIDS+=("$pid")

  # wait for port to open (max 5s)
  for i in $(seq 1 50); do
    if nc -z 127.0.0.1 "$port" 2>/dev/null; then
      ok "$name (PID $pid) on :$port"
      return
    fi
    sleep 0.1
  done
  fail "$name failed to start — check $log_file"
}

start_server zen  zen_server  8081
start_server gin  gin_server  8082
start_server echo echo_server 8083
start_server std  std_server  8084

echo ""
log "All servers ready. Starting load test..."
echo ""

# ── run ───────────────────────────────────────────────────────────────────────
"$BUILD_DIR/runner" "$@"