#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"

# Isolated local state/output ensure this smoke test cannot touch a real user's downloads.
CONFIG_DIR="$ROOT/.smoke-config"
OUTPUT_DIR="$ROOT/.smoke-downloads"
rm -rf "$CONFIG_DIR" "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"
trap 'rm -rf "$CONFIG_DIR" "$OUTPUT_DIR"' EXIT

# TDD regression gate: unit, race, static analysis, build.
go test ./...
go test -race ./...
go vet ./...
go build -o "$ROOT/build/telegram-media-downloader-backend" ./cmd/manager

OUTPUT="$( {
  printf '%s\n' \
    '{"version":1,"id":"health-smoke","action":"health"}' \
    '{"version":1,"id":"chats-smoke","action":"list_chats"}' \
    "{\"version\":1,\"id\":\"download-smoke\",\"action\":\"download_export\",\"params\":{\"exportPath\":\"$OUTPUT_DIR/lowlevel-export.json\",\"directory\":\"$OUTPUT_DIR/lowlevel-downloads\",\"include\":[\"mp4\",\"pdf\"],\"takeout\":true,\"continue\":true}}" \
    "{\"version\":1,\"id\":\"start-download-smoke\",\"action\":\"start_chat_download\",\"params\":{\"chat\":{\"id\":4488394154,\"type\":\"channel\",\"name\":\"Demo Channel\",\"username\":\"demo\"},\"destinationRoot\":\"$OUTPUT_DIR\",\"filter\":\"videos_and_documents\",\"takeout\":true}}"
  sleep 1
} | TDL_PATH="$ROOT/scripts/fake-tdl.sh" TMM_STATE_DIR="$CONFIG_DIR" "$ROOT/build/telegram-media-downloader-backend")"

printf '%s\n' "$OUTPUT"
printf '%s\n' "$OUTPUT" | grep -q '"event":"result"'
printf '%s\n' "$OUTPUT" | grep -q '"event":"download_progress"'
printf '%s\n' "$OUTPUT" | grep -q '"event":"download_completed"'
printf '%s\n' "$OUTPUT" | grep -q '"event":"job_completed"'
if [ ! -f "$OUTPUT_DIR/Demo Channel [4488394154]/Videos/Smoke Test Video.mp4" ]; then
  echo "expected caption-renamed output not found" >&2
  exit 1
fi
echo "Backend protocol + chat-download smoke test passed using fake tdl. This does not replace a real Telegram account smoke test."
