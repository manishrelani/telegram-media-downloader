# Phase 6 — Release-safety hardening status

> Historical implementation checkpoint. For the current GitHub-only publication process and acceptance checks, use [Publishing and releases](RELEASING.md). Statements below about an unavailable Flutter SDK or a separately installed CLI describe that earlier checkpoint.

## Completed in this iteration

- Managed downloads now require a recognized stable `tdl` version at or above `0.20.4`.
- Pre-release versions are rejected even when their numeric version is newer (for example `0.21.0-rc.1`); unknown/development build strings fail closed.
- Backend `health` reports the detected version, minimum safe version, managed-download support, and graceful-interruption capability.
- The backend rejects start/resume/direct-download requests before creating a job when the version is unsupported.
- Flutter controller state displays an actionable update message and disables the start-download action when the backend says the version is unsupported.
- Added regression coverage for old versions, stable minimum/newer versions, pre-release versions, unknown versions, health refresh after upgrade, and rejection before job creation.
- Added a service-level test proving distinct chats with identical display names are separated by chat ID in output paths.
- Corrected the implementation plan to mark bounded transient retries as implemented: at most three attempts with exponential backoff; authorization errors and unparsed rate limits are not automatically retried.

## Validation performed here

The following checks passed after the changes:

```bash
make test
make build
make smoke
GOOS=darwin GOARCH=arm64 go build ./cmd/manager
GOOS=windows GOARCH=amd64 go build ./cmd/manager
GOOS=linux GOARCH=amd64 go build ./cmd/manager
```

The smoke test runs against a fake `tdl` executable. Cross-compilation verifies the Go backend builds for the target OS/architecture combinations; it does not test runtime process-control behavior on those operating systems.

## Still required before public release

- Install the target `tdl` release on macOS and validate its exact version output, chat listing, caption export, real video/PDF downloads, and naming behavior.
- Verify pause/continue on a large real file, including checksums, restart recovery, database-lock handling, and no duplicate final files.
- Run `flutter pub get`, `flutter analyze`, `flutter test`, and macOS desktop build with the real Flutter SDK. Flutter/Dart SDK was not available in this authoring environment, so Dart source has not been compile-verified here.
- Add and test free-space preflight only after a reliable estimate of media size is available; the observed JSON export format does not guarantee per-file sizes.
- Validate Windows/Linux runtime process handling and package/installer behavior on their native runners.
- Complete license/third-party notices, privacy review, release checksums, and signing/notarization before publishing.

## Safety note

The minimum-version gate is a conservative guard based on the upstream fix for a known concurrent map-access issue when interrupted download progress is saved. It does **not**, by itself, prove that pause/resume is reliable across all files, Telegram accounts, or platforms. Keep Pause/Resume marked as not release-verified until the manual tests above pass.
