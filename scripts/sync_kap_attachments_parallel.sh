#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

WORKERS="${WORKERS:-4}"
DELAY="${DELAY:-750ms}"
ERROR_DELAY="${ERROR_DELAY:-15s}"
RETRIES="${RETRIES:-3}"
RATE_LIMIT_SLEEP="${RATE_LIMIT_SLEEP:-20m}"
TRANSIENT_ERROR_SLEEP="${TRANSIENT_ERROR_SLEEP:-20m}"
TRANSIENT_ERROR_THRESHOLD="${TRANSIENT_ERROR_THRESHOLD:-5}"
TIMEOUT="${TIMEOUT:-60s}"
MIN_FREE_BYTES="${MIN_FREE_BYTES:-0}"
PASS_DELAY="${PASS_DELAY:-5m}"
REPEAT="${REPEAT:-0}"
VERBOSE="${VERBOSE:-0}"
STOP_EXISTING="${STOP_EXISTING:-1}"
BUILD_BINARY="${BUILD_BINARY:-1}"
LIMIT_PER_TICKER="${LIMIT_PER_TICKER:-0}"
MAX_RUNTIME_SECONDS="${MAX_RUNTIME_SECONDS:-0}"
FROM="${FROM:-}"
TO="${TO:-}"
OUT="${OUT:-$ROOT_DIR/data/equities/_kap}"
BIN="${BIN:-$ROOT_DIR/.cache/hissebot}"
LOG_DIR="${LOG_DIR:-$ROOT_DIR/data/equities/_kap/parallel_logs/$(date +%Y%m%d_%H%M%S)}"
TICKERS_FILE="${TICKERS_FILE:-$LOG_DIR/tickers.txt}"

usage() {
  cat <<EOF
Usage:
  WORKERS=4 ./scripts/sync_kap_attachments_parallel.sh

Environment:
  WORKERS=4                 parallel worker count
  DELAY=750ms               delay between KAP requests per worker
  RETRIES=3                 retry count per request
  REPEAT=0                  set 1 to repeat passes forever
  STOP_EXISTING=1           stop existing com.hissebot.kap.attachments LaunchAgent before starting
  VERBOSE=0                 set 1 for per-file errors in worker logs
  LIMIT_PER_TICKER=0        optional per-ticker file limit
  MAX_RUNTIME_SECONDS=0     optional soft runtime cap; workers stop between tickers
  FROM=YYYY-MM-DD           optional disclosure start date
  TO=YYYY-MM-DD             optional disclosure end date
  LOG_DIR=...               worker logs directory
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! [[ "$WORKERS" =~ ^[0-9]+$ ]] || [[ "$WORKERS" -lt 1 ]]; then
  echo "WORKERS must be a positive integer, got: $WORKERS" >&2
  exit 2
fi
if ! [[ "$MAX_RUNTIME_SECONDS" =~ ^[0-9]+$ ]]; then
  echo "MAX_RUNTIME_SECONDS must be a non-negative integer, got: $MAX_RUNTIME_SECONDS" >&2
  exit 2
fi

START_EPOCH="$(date +%s)"
DEADLINE_EPOCH=0
if [[ "$MAX_RUNTIME_SECONDS" -gt 0 ]]; then
  DEADLINE_EPOCH=$((START_EPOCH + MAX_RUNTIME_SECONDS))
fi

deadline_reached() {
  [[ "$DEADLINE_EPOCH" -gt 0 && "$(date +%s)" -ge "$DEADLINE_EPOCH" ]]
}

mkdir -p "$LOG_DIR" "$OUT" "$(dirname "$BIN")"

if [[ "$STOP_EXISTING" == "1" ]]; then
  launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.hissebot.kap.attachments.plist" >/dev/null 2>&1 || true
fi

if [[ "$BUILD_BINARY" == "1" || ! -x "$BIN" ]]; then
  echo "Building $BIN"
  go build -o "$BIN" ./cmd/hissebot
fi

find "$ROOT_DIR/data/equities" -mindepth 2 -maxdepth 2 -name kap_disclosures.json -type f \
  | sed -E "s#^$ROOT_DIR/data/equities/([^/]+)/kap_disclosures\\.json#\\1#" \
  | sort -u > "$TICKERS_FILE"

TOTAL_TICKERS="$(wc -l < "$TICKERS_FILE" | tr -d ' ')"
if [[ "$TOTAL_TICKERS" -eq 0 ]]; then
  echo "No tickers found in data/equities/*/kap_disclosures.json" >&2
  exit 1
fi

echo "KAP attachment parallel sync"
echo "root=$ROOT_DIR"
echo "workers=$WORKERS tickers=$TOTAL_TICKERS delay=$DELAY retries=$RETRIES"
if [[ "$MAX_RUNTIME_SECONDS" -gt 0 ]]; then
  echo "max_runtime_seconds=$MAX_RUNTIME_SECONDS"
fi
echo "logs=$LOG_DIR"
echo "tickers_file=$TICKERS_FILE"

run_worker() {
  local worker_id="$1"
  local log_file="$LOG_DIR/worker_${worker_id}.log"
  local pass=1

  while :; do
    if deadline_reached; then
      echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id deadline reached before pass=$pass" >> "$log_file" 2>&1
      break
    fi
    {
      echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id pass=$pass started"
      awk -v workers="$WORKERS" -v worker="$worker_id" '((NR - 1) % workers) + 1 == worker {print}' "$TICKERS_FILE" \
        | while IFS= read -r ticker; do
            [[ -z "$ticker" ]] && continue
            if deadline_reached; then
              echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id deadline reached before ticker=$ticker"
              break
            fi
            echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id ticker=$ticker start"

            args=(
              sync kap-attachments
              -out "$OUT"
              -ticker "$ticker"
              -newest-first
              -delay "$DELAY"
              -error-delay "$ERROR_DELAY"
              -retries "$RETRIES"
              -rate-limit-sleep "$RATE_LIMIT_SLEEP"
              -transient-error-sleep "$TRANSIENT_ERROR_SLEEP"
              -transient-error-threshold "$TRANSIENT_ERROR_THRESHOLD"
              -timeout "$TIMEOUT"
              -min-free-bytes "$MIN_FREE_BYTES"
            )
            if [[ "$LIMIT_PER_TICKER" != "0" ]]; then
              args+=(-limit "$LIMIT_PER_TICKER")
            fi
            if [[ -n "$FROM" ]]; then
              args+=(-from "$FROM")
            fi
            if [[ -n "$TO" ]]; then
              args+=(-to "$TO")
            fi
            if [[ "$VERBOSE" == "1" ]]; then
              args+=(-verbose)
            fi

            if HISSEBOT_COMMAND_TIMEOUT="${HISSEBOT_COMMAND_TIMEOUT:-720h}" "$BIN" "${args[@]}"; then
              echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id ticker=$ticker done"
            else
              status=$?
              echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id ticker=$ticker failed status=$status"
            fi
          done
      echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id pass=$pass completed"
    } >> "$log_file" 2>&1

    if [[ "$REPEAT" != "1" ]]; then
      break
    fi
    if deadline_reached; then
      echo "[$(date '+%Y-%m-%d %H:%M:%S')] worker=$worker_id deadline reached after pass=$pass" >> "$log_file" 2>&1
      break
    fi
    pass=$((pass + 1))
    sleep "$PASS_DELAY"
  done
}

pids=()
for worker_id in $(seq 1 "$WORKERS"); do
  run_worker "$worker_id" &
  pids+=("$!")
done

echo "Started ${#pids[@]} workers."
echo "Follow logs with:"
echo "  tail -f \"$LOG_DIR\"/worker_*.log"

failed=0
for pid in "${pids[@]}"; do
  if ! wait "$pid"; then
    failed=1
  fi
done

echo "All workers finished. logs=$LOG_DIR"
exit "$failed"
