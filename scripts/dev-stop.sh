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
