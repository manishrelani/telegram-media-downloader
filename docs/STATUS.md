# Implementation status

## Current documentation and distribution

The app is now Telegram Media Downloader; the repository name is `telegram-media-downloader`. Historical release archive filenames below retain their original names. Existing local state, manifest names and window-restoration identifiers remain compatible.

Direct GitHub distribution is the intended route. The current user and contributor entry points are the [README](../README.md), [getting started](GETTING_STARTED.md), [development guide](DEVELOPMENT.md), [runtime dependencies](DEPENDENCIES.md) and [release checklist](RELEASING.md). The history below records earlier iterations; their version/test counts and CLI instructions are historical. The patched helper is now v3; current dependencies are pinned in the module files.

Original project code now uses [AGPL-3.0-only](../LICENSE). The public repository is [manishrelani/telegram-media-downloader](https://github.com/manishrelani/telegram-media-downloader). Private reporting/contact details, template app identity and real-device acceptance remain maintainer decisions/checks before an open-source app release. The documentation work does not create a repository or publish a release.

## Storage errors and duplicate detection — 2026-10-28

- [x] Typed storage errors preserve causes and stop finalization at setup/inspection/journal/move failure; completed files and unfinished staging remain recoverable.
- [x] Legacy batch failure results/events retain partial saved/failed outcomes.
- [x] Stable chat-ID folder resolution reuses existing friendly folders across title edits/restart/history removal; ambiguity fails explicitly, no migration or renaming.
- [x] Caption edits preserve saved files; exact raw document/photo content IDs distinguish known replacements independent of thumbnail/filename/size. Old manifests remain compatible; missing/wrong-size files are downloaded again.
- [x] Same-ID destination isolation, concurrent first folder choice, corrupt/symlink roots and renamed-chat legacy resume coverage.
- [x] TDD regressions failed before implementation; fresh independent review findings reproduced/fixed with RED/GREEN. Backend/helper unit/race/vet, Python4, Flutter65/analyzer and all3 protocol smokes pass.
- [x] Extended real-backend synthetic smoke covers metadata edits across restart, same-size content replacement and actual manifest-save failure/stop/retry; no Telegram account actions used.
- [x] Repeatable packaging regression fixes overwriting readonly generated license copies; macOS release built.
- [x] Packaged backend library/thumbnail smokes, v2 identity, strict deep signature and ZIP integrity verified; package: `build/TelegramMediaManager-ReliabilityFixes-macOS.zip`.


## Visible progressive thumbnails — 2026-10-27

- [x] Each ready image streams from the v2 helper through the backend to the UI without waiting for sibling images or process completion.
- [x] Actual viewport requests only; excludes sliver cache/Offstage panes, prioritizes screen order, deduplicates and limits batches to 12.
- [x] Scroll/scope cancellation, partial success caching, busy retry, local error handling and disposal.
- [x] Atomic transfer priority and preview cancellation registration; gated-interleaving regression tests cover both handoff races.
- [x] TDD RED/GREEN evidence in `.superpowers/sdd/visible-thumbnails/progress.md`; 65 Flutter tests and clean analyzer; Go backend/helper unit/race/vet; three Python tests and all three protocol smoke checks pass.
- [x] Independent review complete; identified race fixed and focused preemption tests repeated ten times with race detection.
- [x] macOS release built; packaged backend passes library and streaming smoke tests; v2 identity and app signature verified.
- [x] ZIP integrity verified; package: `build/TelegramMediaManager-ProgressiveThumbnails-macOS.zip` (24,475,215 bytes).


## Sharp thumbnail fix — 2026-10-27

- [x] Diagnosed all 68 sampled local previews as 40×22 embedded placeholders.
- [x] Thumbnail-only tdl helper using the existing login, bounded image requests, exact 64-bit references and versioned locations.
- [x] Progressive image upgrades, persistent cache, snapshot checks, selection/progress preservation and Retina-aware decoding.
- [x] Tiny images retain a clean video placeholder instead of being enlarged into blurry covers.
- [x] Low-priority CLI coordination and process-group cancellation; cached previews remain usable during transfers.
- [x] Limited live check: an actual cached video upgraded from 40×22 to 320×180, with zero video downloads.
- [x] Full 53-test Flutter suite and clean analyzer; backend/helper unit, race and vet checks; all three synthetic protocol smoke checks passed.
- [x] macOS build, both bundled helper checks, app signature and ZIP integrity verified; package: `build/TelegramMediaManager-SharpThumbnails-macOS.zip`.


## Video previews and discovery filters — 2026-10-27

- [x] Validated embedded/cached Telegram JPEG previews and stripped-preview expansion, fixed 16:9 geometry, duration badges, fallback icons and bounded local caching.
- [x] Spoiler suppression, including cache reuse and immediate replacement of an already displayed preview.
- [x] Per-file status/type/size/posted-date/search filters, cross-chat/topic download-history filters, independent sorting and grouping.
- [x] Added Images/Audio categories consistently across Go and Flutter.
- [x] Safe draft/Apply/Escape filter flow, removable restrictions, Clear all and responsive controls.
- [x] Preserved selections with hidden counts/clear action; exact selection types remain correct when view filters change.
- [x] Observational saved-file availability, offline/unknown distinctions and explicit recheck, without mutating completed history.
- [x] Failing-first tests followed by fixes and green verification; all 50 Flutter tests and clean analyzer, Go unit/race/vet, Python tests and both smoke flows passed.
- [x] Light/dark/download previews rendered and inspected; release compilation confirmed.
- [x] Bundled helper smoke, app signature and ZIP integrity verified; package: `build/TelegramMediaManager-ThumbnailsFilters-macOS.zip`.
- [x] Superseded by the sharp thumbnail bridge described above.
- [ ] Live-account preview coverage, native VoiceOver and native picker/file-open acceptance remain manual checks.

## Resizable chat sidebar — 2026-10-27

- [x] Quiet library divider with 220–420px limits and a 248px default; resize cursor and subtle hover/focus feedback.
- [x] Local preference persistence with serialized atomic writes, platform configuration paths and `TMM_STATE_DIR` support.
- [x] Responsive limits preserve 520px for the file pane and retain the preferred width for wider windows.
- [x] Double-click reset, keyboard arrows/Home/End/Enter, secondary-click controls and width semantics.
- [x] Safe handling of missing/corrupt preferences, late restoration and failed saves.
- [x] Clean Flutter analyzer and all 31 tests passed; expanded-sidebar preview rendered and inspected.
- [x] macOS release built; bundled backend synthetic smoke, app signature verification and ZIP integrity check passed.
- [x] Updated app packaged at `build/TelegramMediaManager-ResizableSidebar-macOS.zip`.
- [ ] Native VoiceOver and real-account transfer acceptance remain separate manual checks.

## Quiet library UI iteration — 2026-10-27

- [x] Approved UI/UX Pro Max direction recorded in `design-system/telegram-media-downloader/MASTER.md`.
- [x] Shared semantic colors, light/dark system appearance, readable type scale, focus treatment and reduced-motion indicators.
- [x] Flat file rows, aligned metadata, compact file type menu and count-specific download action.
- [x] Adaptive download tray/full queue, preserved browser scroll and keyboard Find.
- [x] Static loading placeholders, contextual notices and destination validation.
- [x] macOS display name and remembered window geometry.
- [x] `flutter analyze` clean; all 24 Flutter tests passed, including theme contrast, menu/Escape, 150% text, keyboard Find and browser scroll restoration.
- [x] Light/dark/queue previews rendered and inspected; macOS release compiled, bundled helper exercised with synthetic downloads, and app signature verified.
- [x] Go unit/race/vet checks, Python tests and both smoke workflows passed.
- [x] Updated app packaged at `build/TelegramMediaManager-QuietLibrary-macOS.zip`.
- [ ] Native VoiceOver, real-account transfers, and external-volume/native file-picker acceptance remain separate manual checks.


## Current media-library iteration — 2026-10-27

- [x] Topic discovery propagated through `tdl`, Go, JSONL, and Flutter.
- [x] Recent media browser with cached snapshots, Load older, local search/sort, and eight selectable file categories plus All files.
- [x] Exact selection and explicit entire-topic/chat downloads; grouped-album expansion disabled for this workflow.
- [x] Persistent file tasks, stable progress identity, partial-update merging, ANSI normalization, real byte counts when available, and finalization before completion.
- [x] One file per transfer; shared database guard for listing, exports, diagnostics and transfers.
- [x] Topic folders, chat-wide duplicate detection, transactional final paths, missing-output failures, original-destination retry, and restart recovery.
- [x] Flutter analyzer, 19 tests, rendered desktop/compact UI checks, and macOS release compilation.
- [x] Go unit/race tests and vet, Python entitlement tests, original smoke and new isolated library protocol smoke.
- [ ] Real-account topic completeness, nontrivial transfer progress, checksum-verified Pause/Resume, and external-drive/native file-picker validation.
- [ ] Native Windows/Linux runtime validation.

The sections below record the earlier implementation phases. Earlier statements about an unavailable Flutter SDK describe that historical environment; the current checks above supersede them.

## Phase 0 — Backend integration spike

### Completed
- [x] JSON Lines v1 protocol envelope, request IDs, event messages and typed errors.
- [x] `health`, `list_chats`, `export_chat`, `download_export`, `start_chat_download`, `list_jobs`, `pause_download`, `resume_download`, and `cancel_download` actions.
- [x] Safe CLI argument construction without shell interpolation of user-provided paths/selectors.
- [x] `tdl chat ls -o json` parsing and test coverage.
- [x] Progress parser handles carriage-return output and ignores CPU/RAM/goroutine metrics.
- [x] Fake subprocess streaming, integration smoke script and protocol examples.
- [x] Initial integration ADR documenting CLI-vs-Go-package tradeoffs and licensing gate.

### Still pending
- [ ] Real `tdl` version/account smoke test on the target Mac.
- [ ] Pin/document a supported `tdl` release only after compatibility tests pass.
- [ ] Validate a sanitized real export from `--with-content` against the pinned version.
- [ ] Revisit CLI adapter versus direct Go API/fork after real lifecycle testing.

## Layer 1 — Go domain core

- [x] Pure domain models and explicit job-state machine.
- [x] Video/document/all-attachment filters.
- [x] Caption-based filename normalization, extension preservation, Unicode handling, unsafe-character sanitization and collision-safe allocation.
- [x] Per-chat folder path builder, manifest model and ports.
- [x] Unit tests for naming, filters, states, summaries and manifest keys.

## Layer 2 — Adapter / integration

- [x] Parse exported media rows and map `text` to caption/title.
- [x] Filter raw export JSON to selected message/file pairs, preserving unknown upstream fields.
- [x] Map chat-list JSON and download filters to domain types/`tdl` arguments.
- [x] Convert recognized progress text to structured events; normalize common command errors.
- [x] Wire the adapter through the application handler.
- [ ] Validate real `tdl v0.20.4` export and destination naming on the target Mac; ensure output formatting matches the pinned build.

## Layer 3 — Orchestration, queue and persistence

- [x] One-chat workflow: export, filter, stage, download, caption-rename, organize and persist.
- [x] Tests verify that chats with identical display names remain isolated using their chat IDs in output folder names.
- [x] Per-chat `Videos`, `Documents`, `Attachments` folders; video/document default and all-attachment mode.
- [x] Atomic persistent job store and per-chat manifest with restrictive local permissions.
- [x] Skip manifest-completed media only when output file still exists; re-download missing files.
- [x] Collision-safe names and no silent overwrite.
- [x] Same-chat lock and global single-slot guard around `tdl` to avoid local DB collisions.
- [x] Restore persisted jobs after restart and reconcile interrupted states to paused/cancelled.
- [x] `start_chat_download` and `list_jobs` protocol operations.
- [x] Transactional organizer records the intended final filename with status `moving` before moving each file; resume reconciles a final file found in this state and marks it complete rather than downloading a duplicate.
- [x] Bounded retries are implemented for explicitly retryable transient errors (maximum three attempts); permanent authorization failures and unparsed rate limits are not retried automatically.
- [ ] Free-space preflight and job-level discovered-item/byte totals remain open because export metadata does not currently provide a verified size for every media item.

## Layer 4 — Progress and process lifecycle

- [x] Structured progress events and throttling.
- [x] Distinguish Pause from Cancel in application cancellation causes and state transitions.
- [x] On supported Unix platforms, interrupt only the app-owned `tdl` child and wait for its exit.
- [x] Resume uses the same job ID, filtered export/staging directory and `--continue` flag.
- [x] Graceful backend close requests Pause when supported; test confirms a live fake job is persisted as paused before exit.
- [x] Cancellation of inactive paused/queued jobs.
- [ ] Real-file pause/resume and repeated restart test against the exact target `tdl` release on macOS.
- [ ] Windows graceful pause is unsupported by current implementation; define/test platform-specific expectations.

## Layer 5 — Flutter UI/UX source

- [x] Material 3 desktop shell and responsive split layout.
- [x] Connection status, actionable tdl login guidance, refresh and error banner.
- [x] Searchable chat list, type filters, multi-select and select-visible/clear.
- [x] Native destination-directory picker and video/document/all-attachment configuration.
- [x] Queue cards with progress/speed/ETA, state, Pause/Resume/Cancel/Retry controls.
- [x] JSONL client that launches the Go backend, routes structured events, retains diagnostic stderr and does not depend on parsing raw tdl output in Flutter.
- [x] Dart model/controller/widget test source added.
- [ ] Flutter `pub get`, tests, analyzer and desktop build; Flutter/Dart SDK was unavailable in the implementation environment, so UI source is not yet compile-verified.
- [ ] Persist last-used user settings; accessibility/keyboard testing and visual review on macOS remain.

## Layer 6 — End-to-end, security and release

- [x] Go unit tests, race detector, vet, build and fake-protocol smoke test.
- [x] Go cross-compilation for Darwin arm64, Windows amd64 and Linux amd64.
- [x] macOS build helper added with selected-folder entitlement and backend bundling steps.
- [ ] Flutter tests/analyze/build on a Mac with SDK installed.
- [ ] Real Telegram tests (opt-in/manual), full-size downloads, flood-wait and interrupted-resume checks.
- [x] CI workflow added for Go validation across macOS/Windows/Linux and Flutter analysis/tests.
- [ ] Cross-platform process/packaging tests and actual CI runner results remain pending.
- [ ] App signing/notarization, checksums, pinned binary/dependency policy, third-party notices, privacy/license review and release notes.

### Phase 6 — release-safety hardening (current)

- [x] Added semantic version parsing and a fail-closed managed-download version floor of `tdl` v0.20.4.
- [x] Added version capability reporting to backend health: `managedDownloadsSupported`, `minimumSafeManagedDownloadVersion`, and version/platform-gated `supportsGracefulPause`.
- [x] Reject managed start/resume/download requests when the installed `tdl` version is older or unrecognized; user receives a specific upgrade message.
- [x] Flutter state displays the required version and disables the download button if the backend reports an unsupported version.
- [x] Added unit tests for stable, prerelease (including newer prereleases), old, and unknown version strings, plus a backend test proving an unsupported version cannot create a job.
- [ ] Validate the actual version output and behavior against `tdl v0.20.4` on the target Mac.
- [ ] Flutter SDK tests/analyzer/build still require an environment with Flutter installed.

## Verification performed in the authoring environment

Successful commands:

```bash
cd backend
gofmt -w ./cmd ./internal
go test ./...
go test -race ./...
go vet ./...
go build -o ../build/telegram-media-downloader-backend ./cmd/manager
GOOS=darwin GOARCH=arm64 go build -o /tmp/tmm-backend-darwin-arm64 ./cmd/manager
GOOS=windows GOARCH=amd64 go build -o /tmp/tmm-backend-windows.exe ./cmd/manager
GOOS=linux GOARCH=amd64 go build -o /tmp/tmm-backend-linux-amd64 ./cmd/manager
cd .. && ./scripts/smoke.sh
```

The smoke test uses a fake executable and does not prove real Telegram compatibility. `flutter` and `dart` were not present, so no Flutter test/analyze/build result is claimed.
