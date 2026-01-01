#!/usr/bin/env bash
# spawn_backends.sh - Start multiple backend test servers for load balancer testing
#
# Usage:
#   ./spawn_backends.sh              # Starts 5 backends on ports 8081-8085
#   ./spawn_backends.sh 8081 8085    # Custom port range
#
# Each backend runs in the background and logs to scripts/backend-PORT.log
# PIDs are saved to scripts/backend.pids for later shutdown
#
# Requirements: Go toolchain in PATH

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_GO="$HERE/backend.go"
PIDS_FILE="$HERE/backend.pids"

if ! command -v go >/dev/null 2>&1; then
  echo "go toolchain not found in PATH. Please install Go to run backends." >&2
  exit 1
fi

START=${1:-8081}
END=${2:-8085}

echo "Spawning backends from port ${START} to ${END}"
rm -f "$PIDS_FILE"

for port in $(seq "$START" "$END"); do
  echo "Starting backend on port $port"
  # run in background, redirect logs
  nohup go run "$BACKEND_GO" -port "$port" > "$HERE/backend-$port.log" 2>&1 &
  pid=$!
  echo $pid >> "$PIDS_FILE"
  echo "  pid=$pid  log=$HERE/backend-$port.log"
done

echo "Started backends. PIDs written to $PIDS_FILE"
echo "To stop them: $HERE/stop_backends.sh"
