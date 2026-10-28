# Build and develop

[README](../README.md) · [Contributing](../CONTRIBUTING.md)

## Requirements

- Go **1.27.3** (selected by the module files and Makefile).
- Flutter **3.47.6**, matching CI.
- Python 3, Git and Make.
- An Apple Silicon Mac running macOS 13 or later, with Xcode and command-line tools; CocoaPods if required by `flutter doctor -v`.

Follow the official [Flutter desktop setup](https://docs.flutter.dev/platform-integration/desktop) and [Go installation guide](https://go.dev/doc/install).

## Build and run on macOS

```bash
git clone https://github.com/manishrelani/telegram-media-downloader.git
cd telegram-media-downloader
export GOTOOLCHAIN=go1.27.3
make build build-runtime build-thumbnail-helper
python3 scripts/configure_macos_entitlements.py
cd apps/desktop
flutter pub get
TMM_BACKEND_PATH="$(cd ../.. && pwd)/build/telegram-media-downloader-backend" flutter run -d macos
```

Run the commands below from the repository root. Automated tests do not need a Telegram account. For manual account testing, quit the app and authenticate with:

```bash
./build/tdl-tmm login -T qr
```

Use the default tdl namespace. Follow the [tdl login instructions](https://docs.iyear.me/tdl/getting-started/quick-start/) for account prompts and other methods. Keep session data out of the repository.

## Build an app for local use

```bash
./scripts/build_macos_app.sh
open "apps/desktop/build/macos/Build/Products/Release/Telegram Media Downloader.app"
```

The script builds the Flutter app, Go backend, patched tdl runtime and thumbnail helper together, includes dependency sources/notices and signs the complete bundle. All executables target macOS 13.0; the build script and Makefile set `MACOSX_DEPLOYMENT_TARGET` explicitly so a newer local SDK does not raise the runtime requirement. A global tdl installation is not needed.

Maintained macOS host files are included; do not regenerate them during normal setup. The app uses a child-process architecture with App Sandbox disabled. Builds are ad-hoc signed by default; `APPLE_SIGNING_IDENTITY` optionally selects a configured Developer ID. Notarization is not automated. See [Releasing](RELEASING.md) for versioned download packaging.

## Checks

```bash
make test
make test-flutter
make smoke
```

`make test` runs Go unit/race/vet checks and Python tests. `make test-flutter` runs Flutter analysis and unit/widget tests. `make smoke` checks backend interactions with fake processes and isolated state. To race-test the runtime recipe explicitly, use `(cd runtime/tdl && go test -race ./...)`.

Cold builds of the generated Telegram bindings are expensive. On memory-constrained machines, use `GOFLAGS=-p=1 make test` to limit parallel compilation.

Install the pinned scanners to run security checks:

```bash
export GOTOOLCHAIN=go1.27.3
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
go install github.com/zricethezav/gitleaks/v8@v8.30.1
export PATH="$(go env GOPATH)/bin:$PATH"
make security
```

This checks publication paths, secrets, available Git history, imported Go packages and locked hosted Pub dependencies. It fails if a required scan cannot complete; reports stay in ignored `build/`. Publication checks use the working source tree, including unstaged deletions, and reject tracked private/generated files and internal planning notes.

## Source layout

| Path | Purpose |
| --- | --- |
| `apps/desktop/` | Flutter app, tests and macOS host |
| `backend/` | Queue, progress, storage and tdl integration |
| `runtime/tdl/` | Pinned build recipe for the bundled Telegram runtime |
| `thumbnail-helper/` | Thumbnail-only tdl extension |
| `scripts/` | Build, packaging, security and smoke checks |
| `.github/workflows/` | CI, security checks and tag-triggered draft releases |

See the [protocol reference](PROTOCOL.md) and [dependency guide](DEPENDENCIES.md) for integration details.

## Development overrides

| Variable | Purpose |
| --- | --- |
| `TMM_BACKEND_PATH` | Trusted backend override in debug/profile builds |
| `TDL_PATH` | Trusted runtime override in debug/profile builds or standalone backend tests |
| `TMM_STATE_DIR` | Isolated job, preference and cache directory |
| `APPLE_SIGNING_IDENTITY` | Optional signing identity for local macOS builds |

Release apps ignore executable overrides and require the bundled executables. Test/demo fixtures are for development only. Do not put secrets into these variables or commit local state.

## Supported platform

App packaging, CI and security checks currently target macOS on Apple Silicon. Native support for additional platforms can be considered in a future release.
