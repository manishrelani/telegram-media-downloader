# Phase 5 — Flutter desktop UI source

## Implemented

The `apps/desktop` Flutter project provides a Material 3 desktop layout split into a searchable chat browser and download settings/queue. It uses only the local Go JSONL child process; Flutter does not parse the raw `tdl` terminal output.

- Connection status, backend/tdl version label, refresh action and a QR-login command hint for missing auth.
- Chat list adapts `tdl`'s `visible_name` JSON field, searches by name/ID/username, filters channels/groups/private chats and supports multi-select/select visible/clear.
- Native directory selection via `file_selector`; the selected destination must exist and remain mounted.
- File filter controls: videos + documents, videos only, documents only and all attachments; settings include takeout/group/skip-completed.
- Persistent queue hydration from `list_jobs` on startup.
- Queue cards display job state, current file, percent when known, speed, ETA, human-readable failure message, and state-dependent Pause/Resume/Cancel/Retry actions.
- Go protocol messages route a job using the same request ID as the job ID. This identity match is important because the backend request ID becomes the persistent job ID.
- Backend lifecycle on app shutdown requests Pause when graceful pause is supported, waits for job goroutines to persist state, then cancels the parent process context.

## Test source added

- `test/models_test.dart`: chat JSON mapping, invalid IDs, progress event parsing, structured errors and persisted job restoration.
- `test/manager_controller_test.dart`: initialization, sorted accessible chats, selected-chat command parameters, destination validation and job/request ID identity.
- `test/widget_test.dart`: application shell smoke test without spawning a backend process.

## Verification limitation

The environment used to author this project did not contain `flutter` or `dart`, and outbound package downloads were unavailable. Therefore the Dart tests are source-only at this point; no claim is made that `flutter pub get`, `flutter test`, `flutter analyze`, or a desktop build passed. Run those commands on the target Mac before treating this UI layer as verified.

## Next validation gate

1. Generate native host files using `flutter create --platforms=macos,windows,linux .` in `apps/desktop`.
2. Run `flutter pub get`, `flutter analyze`, and `flutter test`.
3. Build and run macOS with the Go backend and `tdl` installed.
4. Validate login error states, directory picker access to external drives, progress updates, app close while downloading, pause/resume, cancel and retry against a real test chat.
5. Fix any SDK/API/analyzer issues found before signing or packaging for public use.
