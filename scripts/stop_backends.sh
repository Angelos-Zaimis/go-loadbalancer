#!/usr/bin/env bash
# stop_backends.sh - Stop all backend servers started by spawn_backends.sh
#
# Usage:
#   ./stop_backends.sh
#
# Reads PIDs from scripts/backend.pids and sends SIGTERM to each process.
# Removes the PID file after stopping all backends.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PIDS_FILE="$HERE/backend.pids"

if [ ! -f "$PIDS_FILE" ]; then
  echo "No pid file found at $PIDS_FILE"
  exit 1
fi

echo "Stopping backends listed in $PIDS_FILE"
while read -r pid; do
  if [ -z "$pid" ]; then
    continue
  fi
  if kill "$pid" >/dev/null 2>&1; then
    echo "killed pid $pid"
  else
    echo "pid $pid not running"
  fi
done < "$PIDS_FILE"

rm -f "$PIDS_FILE"
echo "Done."
