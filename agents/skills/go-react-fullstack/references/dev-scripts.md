# Development Scripts

## `scripts/dev-start.sh`

One-click script to build and start both backend and frontend in development mode.

```bash
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

echo "Building frontend..."
cd "$ROOT_DIR/frontend"
npm run build

echo "Building backend..."
cd "$ROOT_DIR"
CGO_ENABLED=1 go build -o <app-binary> .

echo "Starting backend (port <BACKEND_PORT>)..."
./<app-binary> &
BACKEND_PID=$!
echo $BACKEND_PID > "$BACKEND_PID_FILE"

echo "Starting frontend (port 5173, proxy /api -> localhost:<BACKEND_PORT>)..."
cd "$ROOT_DIR/frontend"
npm run dev &
FRONTEND_PID=$!
echo $FRONTEND_PID > "$FRONTEND_PID_FILE"

echo ""
echo "==================================="
echo "  Backend  : http://localhost:<BACKEND_PORT>"
echo "  Frontend : http://localhost:5173"
echo "==================================="
echo ""
echo "Press Ctrl+C to stop both services."

wait
```

## `scripts/dev-stop.sh`

Stops both services by reading their PID files.

```bash
#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_PID_FILE="${SCRIPT_DIR}/.backend.pid"
FRONTEND_PID_FILE="${SCRIPT_DIR}/.frontend.pid"

STOPPED=0

if [ -f "$BACKEND_PID_FILE" ]; then
  PID=$(cat "$BACKEND_PID_FILE")
  if kill "$PID" 2>/dev/null; then
    echo "Stopped backend (PID: $PID)"
  else
    echo "Backend (PID: $PID) not running"
  fi
  rm "$BACKEND_PID_FILE"
  STOPPED=1
fi

if [ -f "$FRONTEND_PID_FILE" ]; then
  PID=$(cat "$FRONTEND_PID_FILE")
  if kill "$PID" 2>/dev/null; then
    echo "Stopped frontend (PID: $PID)"
  else
    echo "Frontend (PID: $PID) not running"
  fi
  rm "$FRONTEND_PID_FILE"
  STOPPED=1
fi

if [ "$STOPPED" -eq 0 ]; then
  echo "No services running."
fi
```

## Manual Development (alternative)

```bash
# Terminal 1 — Backend
go run .

# Terminal 2 — Frontend (hot reload)
cd frontend && npm install && npm run dev

# Access at http://localhost:5173
# API calls to /api/* are proxied to localhost:9800
```

## Production Build

```bash
cd frontend && npm install && npm run build && cd ..
go build -ldflags="-s -w" -o <app-binary> .
```

Result: a single binary with embedded frontend. Run it anywhere:

```bash
./<app-binary>
# Serves at http://localhost:<BACKEND_PORT>
```

## Notes

- `<BACKEND_PORT>` replacement placeholder — default is 9800 in this project
- `<app-binary>` replacement placeholder — default is project binary name
- `CGO_ENABLED=1` is required for macOS systray (getlantern/systray)
- On Linux without systray support, use `CGO_ENABLED=0`
