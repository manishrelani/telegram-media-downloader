# Layer 2 — `tdl` adapter status

## Implemented and tested with fixtures

- `tdl` subprocess adapter for version, chat listing, message export and JSON-based download.
- Chat JSON to domain `Chat` conversion.
- Export JSON parsing into domain `MediaItem` values; message `text` (enabled by `--with-content`) becomes the preferred caption/title.
- Filtered export creation that preserves unknown root/message JSON fields.
- Video/document/all-attachment filter mapping to the `tdl` CLI.
- Structured progress events with label, percentage, speed and ETA where the CLI exposes them; CPU/RAM/goroutine stats do not become progress events.
- Typed actionable errors for missing `tdl`, local database lock, Telegram flood waits, authentication failures, local storage/permission issues and network failures.
- Unix-only graceful child interruption for both export and download commands; Windows reports that graceful Pause is unsupported.
- Fake-executor tests for command arguments, JSON mapping, progress parsing, error mapping and graceful process interruption.

## Still requires live verification

- Install the selected target `tdl` version on macOS and record its exact version/commit.
- Verify real `tdl chat ls -o json` and `tdl chat export --with-content` output against the fixture assumptions.
- Confirm real output filename prefixes match the organizer's `DialogID_MessageID_` mapping.
- Download a small video and PDF from a test chat and confirm caption mapping, file extension, grouping and output path.
- Pause a nontrivial download, wait for the child process to exit, resume using `--continue`, then verify the file's checksum/playback/PDF integrity and ensure no duplicate output is created.
- Repeat lifecycle behavior on macOS/Linux; Windows graceful Pause is currently intentionally disabled.

These remaining checks need a real `tdl` binary and a Telegram test session. Fake-adapter tests do not prove Telegram compatibility.
