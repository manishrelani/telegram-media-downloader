# Layer 1 — Pure Go domain core

## Goal

Keep business rules independent of Telegram/tdl, filesystem side effects, Flutter, terminal output and networking. This allows most correctness tests to run offline and without credentials.

## Implemented packages

- `backend/internal/domain`: domain models and pure business rules.
- `backend/internal/ports`: interfaces that the application layer will consume and infrastructure adapters will implement.

## Implemented behavior

### Job state machine

Supported states: `queued`, `preparing`, `downloading`, `pausing`, `paused`, `cancelling`, `cancelled`, `completed`, and `failed`. `CanTransition` is the single transition table, and `Transition` returns an error for unknown, identical, or illegal state transitions. Failed/cancelled jobs can be queued for a retry; completed jobs are terminal. This is the domain rule only—process lifecycle semantics must be proven in the adapter/orchestration layers.

### File names

`FilenameFor(caption, originalFilename)` uses a non-empty caption, otherwise falls back to the original basename. It preserves the source extension, avoids duplicating a caption extension that already matches it, normalizes whitespace, replaces cross-platform forbidden characters, handles Windows reserved basenames, truncates very long captions at a UTF-8 rune boundary, and retains Unicode.

`FilenameAllocator` reserves names case-insensitively and assigns deterministic ` (2)`, ` (3)`, ... suffixes. When renaming an existing downloaded file, the caller must exclude that source filename from the existing-name list. The allocator is pure; it does not itself check the real filesystem. The filesystem adapter must provide current names and handle a race between allocation and rename.

### Media filters

`ClassifyMedia` uses a documented extension allowlist for common video and document types. `ShouldIncludeMedia` supports videos + documents, videos only, documents only, and all attachments. Unknown extensions are included only in `all_attachments`; MIME/content inspection is outside this domain layer.

### Destination layout

`BuildChatDirectories` computes `<root>/<safe chat name>/Videos` and `Documents`. It doesn't create directories or claim the destination volume exists; that is the filesystem adapter's responsibility.

### Progress summary

- `ResolvedPercent` counts completed, failed and skipped items as resolved.
- `OverallPercent` estimates successful transfer progress from completed files and known per-file percentages. Failed/skipped items do not count as transferred.
- If an active item has no known percentage, overall transfer progress is `nil`/unknown; an empty job also has unknown percentages instead of dividing by zero or pretending it is 0%.

### Manifest model

The in-memory manifest uses a stable key built from chat ID, message ID and original filename, which lets a later persistence layer distinguish multiple attachments on the same message. `MarkCompleted`, `MarkMoving` and `IsCompleted` define the domain semantics. The filesystem organizer uses a `moving` journal status before renaming; resume reconciles a final file that exists while marked `moving`. Content checksums and broader corruption detection remain pending.

## TDD record

Tests were added first for the public domain behavior. Before implementation, `go test ./internal/domain` failed because the domain symbols did not exist. The implementation was then added and the suite passed. The test suite covers Unicode and long captions, blank-caption fallback, extension handling, unsafe names, case-insensitive collisions, allowed state transitions, invalid transitions, filter behavior, unknown progress, zero-item progress, failed items, per-chat paths and manifest keys.

## Validation commands

Run from `backend/`:

```bash
go test ./...
go test -race ./...
go vet ./...
```

The domain tests are offline and require no Telegram account.

## Not part of Layer 1

- Reading/exporting actual Telegram messages.
- Parsing live export JSON into `MediaItem` values.
- Renaming real downloaded files.
- Persisting job/manifest state to disk.
- Download scheduling, retries or pause/resume process control.
- Flutter UI.
