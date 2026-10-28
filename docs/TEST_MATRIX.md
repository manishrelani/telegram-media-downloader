# Test and validation matrix

## Automated checks in this repository

| Layer | Tests | Requires Telegram account? | Current validation |
|---|---|---:|---|
| Domain | Naming/sanitization, collision suffixes, media filters, job state transitions, progress aggregation, manifest keys, path layout | No | Executed locally; passing |
| tdl adapter | Chat JSON mapping, export caption parsing, filtering while preserving unknown fields, command arguments, progress parsing, typed errors, managed-download version support, process interruption | No for fakes; yes for real behavior | Fakes and unit tests executed; live behavior pending |
| Persistent store | Atomic JSON state, restart loading, corruption/version checks, cancellation, private permissions | No | Executed locally; passing |
| Filesystem organizer | Caption rename, video/document/other folders, duplicate names, missing/ambiguous matches, no-overwrite behavior | No | Executed locally; passing |
| Application service | One-chat workflow, filter behavior, resume state, repeated runs, missing files, error paths, cancellation, recovery, concurrency | No with fake adapter | Executed locally; passing |
| Protocol | JSON Lines request handling, start/list/cancel/pause/resume flows, old-tdl-version refusal | No with fake adapter | Executed locally; passing |
| Fake integration smoke | Launch compiled backend and fake `tdl`, issue JSONL commands, verify caption-renamed file | No | `scripts/smoke.sh` executed; passing |
| Flutter models/controller/widget | Parsing, chat list, destination validation, UI smoke | No with fake backend | Quiet library tests cover theme contrast, multi-select menus, Escape dismissal, 150% text, keyboard Find and scroll restoration; final results are recorded in STATUS.md |
| Topic/media library | Exact topic/file selection, file type filters, partial progress, missing outputs, pause/restart/resume, retry, snapshot refresh and compiled JSONL smoke | No | Unit/race checks and `scripts/library_smoke.py` passed |
| macOS release build | Flutter compilation and bundled Go helper | No | Build passed; live account/native interactions pending |
| Real Telegram smoke | Sign-in, real chat listing/export, video/PDF download, title mapping | Yes | Pending on target Mac |
| Pause/Resume real-file test | Interrupt a nontrivial download, restart/resume, compare checksums, verify no duplicate/partial final file | Yes | Pending against a pinned `tdl` release |
| Release tests | macOS external-drive picker/sandbox access, code signing/notarization, upgrades, Windows/Linux packaging | Platform-specific | Pending |

## Local backend verification

From the repository root:

```bash
make smoke
```

This executes Go unit tests, race detection, vet, build, and a fake-`tdl` end-to-end protocol workflow.

For Flutter, after installing the Flutter SDK:

```bash
cd apps/desktop
flutter create --platforms=macos,windows,linux .
flutter pub get
flutter analyze
flutter test
```

## Live smoke checklist (manual)

1. Install the supported `tdl` version and authenticate with a dedicated test account.
2. Confirm chat listing for a public channel, private channel, group and private conversation the account can access.
3. Download a small MP4 and PDF whose Telegram captions differ from the uploaded filenames.
4. Verify the output titles, extensions, per-chat directories and duplicate-caption suffixing.
5. Pause a large file and wait for the UI to show `Paused`; verify the Go child exited and the local tdl database is not locked.
6. Resume the same job, check the final file checksum/playback/PDF integrity, and confirm no duplicated file appears.
7. Close the app during download and reopen; check a recoverable job appears and can resume.
8. Remove a previously completed output file and rerun with `Skip completed` enabled; verify the missing file is downloaded again.
9. Unmount the destination drive before starting a job; verify the app refuses to start and does not create files on the internal disk.
10. Repeat platform-specific process-control tests on Windows/Linux before advertising their Pause/Resume support.

## Topic and file-level live acceptance

- Open a forum group; compare the displayed topics (including General) with Telegram.
- Select a topic with media, then PDFs only; verify only that topic's PDFs transfer.
- Select an individual file from a recent page; ensure no album siblings or older files enter the job.
- Load older results; verify deduplication and selection remain correct after refresh.
- Verify initial/partial/stale progress on a real large file; CPU metrics must not become file progress.
- Pause, close, reopen, resume, and checksum-compare; completed files must not transfer again.
- Verify topic folder naming, cross-scope duplicate skipping, Open file/Show in folder, and disconnected destinations.
- Confirm a deleted/unavailable requested attachment fails rather than silently completing.

## Quiet library UI acceptance

Automated checks cover normal text contrast on main/sidebar surfaces and primary actions in both themes, file-type multi-selection and Escape, compact layouts at 150% text, and queue/browser scroll plus keyboard search continuity. Rendered sample-data previews cover light, dark, and download queue layouts. Static skeletons and activity indicators honor reduced-motion preferences.

Manual macOS checks still include VoiceOver navigation, the native folder picker and file-open actions, and a real account transfer. These are distinct from rendered widget and synthetic-backend checks.

## Resizable sidebar acceptance

Automated checks cover actual preference-file round trips across new store instances, serialized writes and unknown-key preservation, corrupt preferences, UI restoration, both drag limits, double-click reset, responsive width restoration without overwriting preferences, compact drawer behavior, keyboard controls, secondary-click menu controls, late-load race handling and failed-save feedback. Existing light/dark, larger-text and library/queue checks remain active. A synthetic preview records the expanded sidebar.

On macOS, manually drag the divider with a mouse or trackpad, double-click to reset, close/reopen the app and confirm the chosen width returns. Reduce the window width and then enlarge it to confirm the preferred width returns. Native VoiceOver interaction remains a separate manual check.

## Preview and filter TDD coverage

| Behavior | Evidence |
|---|---|
| Cached/stripped JPEGs, malformed base64, absent/vector previews, invalid dimensions/version and spoilers | `backend/internal/tdl/thumbnail_test.go` |
| Present/missing/offline/legacy saved files, observation without history mutation | `backend/internal/application/availability_test.go` |
| Combined status/type/chat/topic/search restrictions, each size boundary, inclusive local date ranges and unknown metadata | `apps/desktop/test/media_filters_test.dart` |
| Group identities with duplicate names, all grouping modes, size-sort unknowns | `apps/desktop/test/media_filters_test.dart` |
| Persistent/corrupt cache, oversized inputs, fixed preview geometry, immediate spoiler suppression | `apps/desktop/test/media_experience_test.dart` |
| Per-file queue filters, Escape cancellation, date-picker cancellation, Apply, Clear all, compact large text, grouping after collapse | `apps/desktop/test/media_experience_test.dart` |
| Hidden selection retention/disclosure and exact selected types in protocol requests | `apps/desktop/test/library_test.dart`, `media_experience_test.dart` |
| Thumbnail metadata without transfers, restart preservation and deleted saved-file observations | `scripts/library_smoke.py` |

The cycles began with missing-feature failures. Behavioral failures then exposed a cache future resolving to itself, stale images while replacing a preview, filters changing the download type allowlist for hidden selected files, and overly tall compact controls. Fixes were followed by the focused tests and the full suite. The existing sidebar, contrast, keyboard, queue context and protocol checks remain active.

Live-account preview coverage, Telegram rate behavior, native VoiceOver and native picker/file-open acceptance remain manual checks. Synthetic previews use a generated test slide and never imply a real-account transfer was exercised.

## Sharp thumbnail regression checks

- `backend/internal/tdl/thumbnail_reference_test.go`: exact int64 IDs, a Retina-sized image preference, private references excluded from UI JSON.
- `thumbnail-helper/main_test.go`: empty/vector/video thumbnail types rejected before API calls, bounded `upload.getFile` image requests, malformed/oversized responses rejected.
- `backend/internal/application/thumbnail_test.go`: upgrades and disk reuse, unknown snapshots, no network work alongside an active transfer, transfer preemption, cached images during transfers.
- `backend/internal/tdl/preview_process_test.go`: interrupt the owned parent and child group and allow cleanup to complete.
- `apps/desktop/test/thumbnail_quality_test.dart`: snapshot scoping, progress/selection retention, later job events, Retina decode sizes and no enlargement of tiny placeholders.
- `scripts/thumbnail_smoke.py`: real backend JSONL upgrade event, exact thumbnail request, disk-cache reuse and zero video transfers with a synthetic tdl.
- Limited live diagnostic: one existing cached video upgraded from 40×22 to 320×180; no full-video download ran. This validates the new thumbnail path, separately from the existing real-download/manual acceptance list.
