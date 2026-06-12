#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PID_DIR="${ROOT_DIR}/scripts"
BACKEND_PID_FILE="${PID_DIR}/.backend.pid"
FRONTEND_PID_FILE="${PID_DIR}/.frontend.pid"

cleanup() {
  echo "Cleaning up on exit..."
  [ -f "$BACKEND_PID_FILE" ] && kill "$(cat "$BACKEND_PID_FILE")" 2>/dev/null && rm "$BACKEND_PID_FILE"
  [ -f "$FRONTEND_PID_FILE" ] && kill "$(cat "$FRONTEND_PID_FILE")" 2>/dev/null && rm "$FRONTEND_PID_FILE"
  exit 1
}
trap cleanup SIGINT SIGTERM

echo "Starting backend (port 9800)..."
cd "$ROOT_DIR"
go run . &
BACKEND_PID=$!
echo $BACKEND_PID > "$BACKEND_PID_FILE"

echo "Starting frontend (port 5173, proxy /api -> localhost:9800)..."
cd "$ROOT_DIR/frontend"
npm run dev &
FRONTEND_PID=$!
echo $FRONTEND_PID > "$FRONTEND_PID_FILE"

echo ""
echo "==================================="
echo "  Backend  : http://localhost:9800"
echo "  Frontend : http://localhost:5173"
echo "==================================="
echo ""
echo "Press Ctrl+C to stop both services."

wait
