# Telegram Media Downloader — Layered Implementation Plan (TDD)

**Status:** Living implementation plan. Go protocol/domain/adapter/orchestration and a tested fake-process lifecycle foundation are implemented. Flutter UI source and its Dart tests are now added, but Flutter SDK validation and real Telegram pause/resume tests remain open.

## Product goal

Build a local-first, cross-platform desktop application using **Flutter (UI) + Go (backend)**. Users authenticate through their own `tdl`/Telegram session, browse chats available to that account, select one or more chats, download video/document media, and save files into per-chat folders using message captions as filenames where possible.

## Product principles

- Local-first: no app account, no server, and no upload of chat data or Telegram sessions.
- Safe by default: do not overwrite files, do not kill unrelated processes, and never show a job as paused until the downloader has stopped safely.
- TDD: write a failing test first for each behavior, implement the smallest change, run tests, refactor, then continue.
- Observable: user-facing status/progress goes to Flutter; diagnostic logs go to stderr/a rotating local log file, not the UI event stream.
- Honest progress: distinguish current-file progress from overall job progress; show unknown values as unknown rather than inventing them.
- Platform-aware: macOS first, then Windows and Linux with explicit process-lifecycle testing on each.

## Architecture proposal

```text
Flutter desktop UI
  └── versioned JSON Lines protocol (stdin/stdout)
        └── Go application service / job manager
              ├── domain and use cases
              ├── persistence (settings, jobs, manifests)
              ├── filesystem adapter (naming, safe paths, collision handling)
              └── Telegram/tdl adapter
                    └── tdl implementation or managed tdl subprocess
```

### Suggested repository layout

```text
telegram-media-downloader/
  apps/desktop/                 # Flutter desktop app
    lib/app/
    lib/core/
    lib/features/account/
    lib/features/chats/
    lib/features/downloads/
    test/
  backend/
    cmd/manager/main.go          # executable entry point
    internal/domain/             # Chat, MediaItem, DownloadJob, state machine
    internal/application/        # use cases / orchestration
    internal/ports/              # interfaces for Telegram, download, storage, clock
    internal/adapters/tdl/       # tdl integration
    internal/adapters/fs/        # safe paths, naming, disk checks
    internal/transport/jsonl/    # stdin/stdout protocol
    internal/store/              # settings/job persistence
    go.mod
  testdata/                      # sanitized tdl JSON fixtures and output fixtures
  docs/                           # architecture decisions, protocol, release checklist
```

## Critical architecture spike before committing to implementation

The current `tdl` download package has an exported `Run` function, but it also renders terminal progress and contains interactive resume behavior. It is therefore not a ready-made headless desktop API. Official docs show JSON chat listing (`tdl chat ls -o json`), JSON-based downloads, and continuation support, but GUI-grade typed progress and graceful pausing require validation.

Compare these two approaches with a small proof of concept:

1. **Managed CLI process:** Go runs a pinned `tdl` binary, parses supported output, sends SIGINT/termination only to its own child, and resumes with `--continue`. Pros: lower coupling, separate dependency. Cons: progress parsing can be brittle and users may need a separate `tdl` install.
2. **Go integration/fork/adapter:** reuse or adapt `tdl` Go packages to expose structured progress events and noninteractive methods. Pros: stronger event/progress control and smoother packaging. Cons: tighter coupling to `tdl` internals, more maintenance, and AGPL-3.0 distribution/source obligations that must be reviewed.

**Decision gate:** a fake-adapter end-to-end test now covers chat listing, caption export, staged download, structured progress, graceful interrupt and continuation flags. Flutter UI source can be developed against that stable protocol, but release approval and exposing Pause/Resume as a guaranteed feature remain gated on a real-file test with an exact pinned `tdl` release. Do not depend silently on `master`.

## Layer 0 — Product requirements and protocol contract

### Tasks
- [x] Define MVP behavior and explicit non-goals.
- [x] Define a versioned JSON Lines protocol between Flutter and Go.
- [x] Specify request IDs, job IDs, event names, error codes, and protocol version negotiation.
- [x] Keep stdout exclusively for protocol JSON; send diagnostic logs to stderr/log files.
- [x] Define job state transitions: `queued`, `preparing`, `downloading`, `pausing`, `paused`, `cancelling`, `cancelled`, `completed`, `failed`.
- [x] Write an ADR documenting the provisional tdl integration strategy and licensing decision gate.

### Acceptance criteria
- Protocol examples are checked into the repository.
- Unknown protocol versions/actions produce a typed error, not a crash.
- No Telegram session values or private message content are written to ordinary logs.

### TDD cases
- Valid command decodes and validates.
- Malformed JSON returns a protocol error and does not kill the backend.
- Unsupported protocol version returns a compatibility error.
- Multiple requests do not have their responses/events confused (request/job IDs remain stable).

## Layer 1 — Go domain “brain” (pure logic, no Telegram or UI dependencies)

### Tasks
- [x] Implement domain types for Chat, MediaItem, DownloadJob, DownloadRequest, ProgressEvent/Summary, and AppError.
- [x] Implement an explicit, validated job-state machine.
- [x] Implement file classification (video/document/all attachments) separately from downloading.
- [x] Implement caption-to-filename normalization and collision-safe naming.
- [x] Implement per-chat destination layout and a manifest data model for resumability/idempotency. Disk-backed persistence remains a later layer.
- [x] Define interfaces for chat source, exporter, downloader, filesystem, and job store.

### Required unit tests (table-driven)
- Caption with ordinary text becomes the filename while preserving the source extension.
- Empty/whitespace caption falls back to original filename.
- Caption with Unicode, emoji, punctuation and non-Latin scripts remains usable.
- Path separators, control characters, reserved names, trailing dots/spaces and very long captions are sanitized/truncated safely.
- Duplicate titles get deterministic suffixes; existing files are never overwritten.
- File extension is preserved and never guessed as `.mp4` for every item.
- Video/document filters accept expected extensions and handle unknown extensions according to the selected mode.
- Invalid job-state transitions are rejected (e.g. completed → paused).
- Overall progress handles zero items, unknown sizes, failed items, and completed/queued items without division by zero.
- [x] Existing manifest prevents duplicate re-downloads when a job is run again, provided the recorded output file still exists.

### Acceptance criteria
- Domain tests run without Telegram, disk downloads, Flutter, or network access.
- Core package has no dependency on Flutter or terminal rendering.

**Implementation status:** The pure domain package and ports are implemented and tested. The `tdl` adapter maps chat/export models and structured progress in tests; caption filtering, staging downloads, post-download organization, disk-backed job persistence and manifest-based deduplication now run in the fake end-to-end path. Real Telegram behavior remains unverified.

## Layer 2 — Go adapters and tdl integration

### Tasks
- [x] Detect `tdl` availability/version with `health`; return actionable errors when the executable is unavailable.
- [x] Detect whether the active session can list accessible chats; do not infer login solely from files existing.
- [x] Map `tdl chat ls -o json` output into domain Chat objects through the domain adapter.
- [x] Export message content/captions using documented export options; parse exported file rows into domain MediaItem values.
- [x] Download selected JSON exports with requested filters/destination and continuation flags through the domain adapter (real continuation behavior remains unverified).
- [x] Normalize common subprocess, stderr, authentication/database-lock, network, rate-limit and storage errors; lifecycle handling remains provisional.
- [x] Never automatically kill all `tdl` processes; the handler only cancels and waits for jobs started by this application.

### TDD and adapter tests
- Parse valid chat JSON fixture with channels, groups, bots and private chats.
- Handle empty list, missing optional fields, unknown chat type and malformed JSON.
- Build CLI arguments safely for paths/spaces and do not pass user input through a shell string.
- Map login-required, network, permission, missing binary and database-lock failures to actionable errors.
- Verify the `--with-content` export fixture maps captions correctly.
- Verify selected output formats/options produce expected command arguments.
- Ensure stdout protocol remains valid even when tdl emits noisy progress; all raw output is captured separately.

### Acceptance criteria
- Adapter tests use fake executables and fixtures; they do not require a real Telegram account.
- One opt-in manual smoke test confirms behavior against the pinned tdl build and a test chat.

## Layer 3 — Download orchestration, queue and persistence

### Tasks
- [ ] Add an atomic batch request (the Flutter UI currently starts one job request per selected chat; the Go service globally serializes `tdl` work).
- [x] Export before download, preserve the job-specific work area if interrupted, and persist a manifest for diagnosis/deduplication.
- [x] Download videos and documents by default; support all attachments as an optional mode.
- [x] Organize files as `<destination>/<safe chat name> [<chat ID>]/Videos`, `/Documents` and `/Attachments`.
- [x] Persist job status and completed file mapping using atomic file writes.
- [x] Implement bounded retries (maximum three attempts with exponential backoff) for transient failures; do not retry permanent authorization errors or flood-waits automatically.
- [x] Reject non-existent destination roots before starting to help catch unmounted external drives; free-space preflight and in-transfer monitoring remain pending.
- [x] Report per-item renamed/skipped/failed/already-completed counts; discovered/supported/bytes totals remain pending.

### TDD cases
- [x] One chat with videos and PDFs creates the correct folder layout.
- [x] Duplicate captions in a chat get collision-safe suffixes.
- [x] Add an explicit test for two different chats with identical names; folder includes the chat ID.
- [ ] Failure in one chat does not silently mark a whole batch successful (batch orchestration is not implemented).
- Rerunning a completed job skips verified existing files by manifest/fingerprint policy.
- [x] A `moving` manifest journal record is persisted before each final file move; resume reconciles the moved-file/manifest-completion crash window without re-downloading the file.
- External drive missing before start prevents downloads and offers a clear error.
- Retry count is bounded and cancellation stops future retry attempts.

## Layer 4 — Progress, Pause, Resume, Cancel and process lifecycle

### Current progress
- [x] Typed progress and job-state events are exposed to the JSONL client.
- [x] The backend throttles low-level progress events, drains active job goroutines during shutdown, and requests Pause on supported platforms before parent cancellation.
- [x] Pause/Resume/Cancel state transitions and startup recovery have fake-adapter tests.
- [x] Unix child process interruption is scoped to the child created by this app; Windows reports that graceful Pause is unsupported.
- [ ] Real-file Pause/Resume and child-exit behavior remain unverified against the pinned `tdl` build.

### Important semantics
Pause is not “hide progress”, and should not suspend a process with an OS-level freeze. It is a request to stop the current download gracefully, wait until the backend confirms the process/operation has exited and checkpointed state, then show `paused`. Resume starts from the saved state using the verified continuation mechanism. If the underlying download engine cannot guarantee clean pause at the current file, the UI must say “Pausing…” and only show “Paused” when safe.

### Tasks
- [x] Define typed `ProgressEvent` and job-state JSONL events.
- [x] Throttle low-level progress events while preserving terminal/error events.
- [x] Distinguish Pause vs Cancel causes and persist job state before terminal events.
- [x] On restart, recover interrupted jobs and allow Resume/Cancel from the persisted queue.
- [x] Implement platform-specific child interruption; Windows does not advertise graceful Pause.
- [ ] Real-file Pause/Resume on macOS/Linux and recovery after app crash must pass manual tests before release.

### TDD cases
- Progress increases monotonically for a single attempt where the engine supplies such data; reset is explicit on retry.
- UI event rate is bounded (for example, no more than 5 progress updates per second per active job) while terminal events are never dropped.
- Pause during queued/preparing/downloading states has deterministic results.
- Resume after pause restarts the exact job and does not duplicate completed files.
- Cancel while downloading preserves completed files and stops queued items according to documented policy.
- Repeated pause/resume/cancel calls are idempotent or return typed invalid-state responses.
- Backend shutdown requests graceful cancellation and waits a bounded time before its own child is force-stopped.
- Simulate interruption and verify checkpoint + continuation; add manual integration test for real tdl behavior.
- Run Go race detector on queue/event/state tests (`go test -race ./...`).

### Acceptance criteria
- Pause/Resume is not declared release-ready until interruption/resume behavior is verified with the exact pinned tdl version on each supported OS.
- A database lock from another app or process produces a useful message and never triggers broad process termination.

## Layer 5 — Flutter UI/UX (after backend contract is stable)

### Screens/components
- [x] Connection state: tdl availability, chat-list-based session check, retry and QR login instructions.
- [x] Chat browser: search, type filters, multi-select, select-visible/clear, refresh and visible-chat metadata.
- [x] Download configuration: native destination picker, media type, group/takeout/skip-completed settings.
- [x] Queue cards: caption-aware current filename, per-file progress, speed/ETA, Pause/Resume/Cancel/Retry.
- [x] Empty, loading, disconnected and error states.
- [x] JSONL backend client routes structured events; raw tdl progress text stays in Go.
- [ ] Job details/log drawer, Open Folder, persisted UI preferences and overall item/byte counts.
- [ ] Keyboard navigation, focus management, screen-reader labels, scalable text and contrast testing.

### Flutter tests
- [x] Added model tests for tdl chat JSON, progress decoding, backend errors and persisted jobs.
- [x] Added controller tests for chat/job loading, selected chat, file filter, destination validation and download request parameters.
- [x] Added a widget smoke test for the primary screen.
- [ ] Execute `flutter test` and `flutter analyze` on a machine with Flutter 3.32+ installed; current environment has no Flutter SDK.
- [ ] Expand tests for search/filter, multi-select, specific job progress routing, error states and Pause/Resume/Cancel control visibility.
- Golden tests for key layout states where useful; avoid making all tests brittle visual snapshots.

## Layer 6 — End-to-end, security, release and maintenance

### Test matrix
- [x] Unit tests: Go domain/backend; Flutter state/controller test source added but not executed.
- [ ] Contract tests: JSON Lines protocol fixtures shared between Go and Flutter.
- [ ] Adapter tests: fake tdl executable and sanitized fixtures.
- [ ] Integration tests: temporary local directory, representative video/PDF fixture, duplicate names, interruption/restart.
- [ ] Opt-in Telegram smoke tests with a test account/chat only; never require user session secrets in CI.
- [ ] Performance tests: large chat list, thousands of exported messages, many queue entries, event throttling and memory usage.
- [x] Go backend cross-compiles for Darwin arm64, Windows amd64 and Linux amd64.
- [ ] Full Flutter/process/filesystem/signing test matrix on macOS, Windows and Linux.
- [ ] CI: `gofmt`, `go vet`, `go test ./...`, `go test -race ./...` where supported, `flutter analyze`, `flutter test`, and release build checks.
- [ ] Supply-chain/security: pin dependencies, generate SBOM if packaging, verify downloaded binaries/checksums, scan secrets, keep tokens out of logs.
- [ ] Add LICENSE, third-party notices, privacy policy, user documentation, release notes, checksums, and a supported-version policy.
- [ ] Review `tdl` AGPL-3.0 obligations and Telegram API terms before publishing binaries or deciding whether the product is proprietary.

## Definition of Done for every task

1. A test is added first and fails for the expected reason (Red).
2. The smallest implementation makes it pass (Green).
3. Refactor with tests remaining green (Refactor).
4. Run all relevant tests and static analysis.
5. Update docs/protocol if behavior or state transitions changed.
6. Include failure cases and user-facing error mapping, not only happy-path behavior.
7. No task is marked done solely because it compiles.

## MVP acceptance criteria

- User can open the desktop app and see whether backend/tdl is available and authenticated.
- User can search and select one or more accessible chats.
- User can choose video + document downloads and a destination directory.
- Downloaded media is organized per chat and uses caption/message text as filename when available, preserving actual file extension.
- Files are never overwritten silently; duplicate title handling is deterministic.
- UI shows current item and aggregate progress without terminal-output flooding.
- Pause, Resume and Cancel have truthful, tested state transitions; a job can be recovered after app restart.
- Failures are visible and actionable, with logs that exclude session secrets.
- Tests run without real Telegram credentials; network-based tests are opt-in only.

## Risks / scope decisions to close before implementation

1. **tdl integration method and license:** choose CLI adapter versus Go package/fork after the spike. A direct package integration may create stronger coupling and has licensing consequences; subprocess separation is not automatically a legal exemption.
2. **Pause granularity:** pause may mean interrupt current download and resume remaining work, not freeze/resume at the exact byte. Confirm what the pinned downloader can reliably restore.
3. **MVP platforms:** recommend macOS first, then Windows and Linux after process control and packaging tests.
4. **Media completeness:** decide whether default mode includes only videos and documents or every attachment. Unknown extensions need an explicit policy.
5. **File naming:** use message caption first; fall back to original filename when empty; preserve actual extension and prevent traversal/overwrite.
6. **Telegram constraints:** only access chats/media available to the signed-in account; explain rate limits and permission/protected-content limits rather than promising unlimited access.
7. **Distribution:** decide project license and public/commercial goal before packaging; include required notices.

## Suggested delivery order

**Milestone 0:** architecture + tdl integration spike + ADR (fake integration is done; real account remains a gate).

**Milestone 1:** Go domain/backend brain, protocol, unit and adapter tests (implemented).

**Milestone 2:** one-chat queue, job store, manifest, safe filenames, filtering, organization and manifest-move crash-window recovery (foundation implemented); real process-resume proof remains open.

**Milestone 3:** the protocol is now wired to Flutter UI source; fake process lifecycle and recovery tests are in place. Next gate is Flutter SDK tests/build plus live macOS account validation.

**Milestone 4:** real-file integration tests, packaging/signing, CI, accessibility, documentation and licensing review.
