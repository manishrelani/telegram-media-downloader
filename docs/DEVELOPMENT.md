# Development

[Back to the README](../README.md) · [Contribution workflow](../CONTRIBUTING.md)

## Toolchain

| Tool | Current build/check configuration |
| --- | --- |
| Go | 1.27.3; module files and Makefile select this toolchain |
| Flutter | 3.47.6 in CI and the latest local validation; pubspec declares a lower minimum |
| Python | Python 3 for packaging, publication and smoke-test scripts |
| Native macOS tools | Xcode and its command-line tools; CocoaPods if required by your Flutter setup |
| Git and Make | Checkout, source publication and root build targets |

Follow the official [Flutter desktop setup](https://docs.flutter.dev/platform-integration/desktop) and [Go installation guide](https://go.dev/doc/install). Run `flutter doctor -v`. Align your Flutter version with `.github/workflows/ci.yml` before diagnosing dependency failures.

Clone the public repository:

```bash
git clone https://github.com/manishrelani/telegram-media-downloader.git
cd telegram-media-downloader
```

The following commands run from the checkout root unless stated otherwise. Normal development edits this checkout directly; source archives are optional release exports.

## Build and run on macOS

```bash
export GOTOOLCHAIN=go1.27.3
make build build-runtime build-thumbnail-helper
python3 scripts/configure_macos_entitlements.py
cd apps/desktop
flutter pub get
TMM_BACKEND_PATH="$(cd ../.. && pwd)/build/telegram-media-downloader-backend" flutter run -d macos
```

The maintained macOS host files are already included. Do not regenerate them as a normal setup step. If your checkout genuinely lacks them, follow the packaging script's host-generation step and review the generated changes.

The three Go executables are built together under `build/`. The backend and UI choose their sibling `tdl-tmm` runtime by default. The app registers its versioned preview helper locally on first use. A global CLI installation is not needed for this configuration.

Release apps require the backend/runtime inside their bundle and ignore developer executable overrides. A missing bundled executable is an installation error; a release app never falls back to a development checkout or fake runtime.

Automated tests use synthetic data. To test against Telegram manually, authenticate using `./build/tdl-tmm login -T qr` with a dedicated test account. Close the app and other tdl processes while authenticating. Keep the session outside version control.

## Build a macOS app bundle

```bash
./scripts/build_macos_app.sh
```

The script builds all three Go executables and the Flutter release, bundles matching dependency sources/notices, removes obsolete preview executables and signs the result. Flutter's output is under `apps/desktop/build/macos/Build/Products/Release/`.

By default the app is ad-hoc signed. `APPLE_SIGNING_IDENTITY` optionally selects a configured Developer ID; notarization is a separate step. Direct GitHub distribution can use an honestly labeled ad-hoc-signed build. See [Releasing](RELEASING.md).

The app is designed for direct desktop distribution, with App Sandbox disabled for this process architecture. The entitlement script preserves the other configured keys.

## Repository map

| Path | Responsibility |
| --- | --- |
| `apps/desktop/lib/` | Flutter UI, controllers, models, backend client and thumbnail scheduling |
| `apps/desktop/test/` | Unit/widget tests and synthetic preview fixtures |
| `backend/cmd/manager/` | JSONL backend executable and runtime selection |
| `backend/internal/` | Application services, domain, rooted filesystem/storage adapters and tdl integration |
| `runtime/tdl/` | Pinned build recipe and security regression for the app-owned Telegram runtime |
| `thumbnail-helper/` | Versioned thumbnail-only tdl extension |
| `scripts/` | Build, smoke, security and public-source packaging tools |
| `testdata/` | Synthetic backend fixtures |
| `.github/workflows/` | CI and security workflow configuration |

[Protocol](PROTOCOL.md) · [Media behavior](MEDIA_LIBRARY.md) · [Runtime dependencies](DEPENDENCIES.md)

## Checks

```bash
make test
make test-flutter
make smoke
```

`make test` runs backend unit/race/vet checks, Python tests, preview-helper unit/race/vet checks and runtime regression/vet checks. For an explicit runtime race check, run `(cd runtime/tdl && go test -race ./...)`. The generated Telegram bindings make a cold Go build relatively expensive; allow it to populate the cache.

`make smoke` runs the JSONL backend against fake processes and isolated local state. It does not use your Telegram account. UI changes also need manual checks for compact windows, large text, light/dark appearance, focus and keyboard navigation.

To run the security gate, put the installed tools on PATH:

```bash
export GOTOOLCHAIN=go1.27.3
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
go install github.com/zricethezav/gitleaks/v8@v8.30.1
export PATH="$(go env GOPATH)/bin:$PATH"
make security
```

Use the pinned Go toolchain to build the scanner too. The gate checks exact publication candidates, available Git history, imported Go packages and locked hosted Pub packages. It fails on unavailable tools, incomplete queries or applicable findings. Reports stay in ignored `build/`. New top-level project files must match the public-source policy in `scripts/public_source.py`; update that policy and its tests intentionally when adding a legitimate new publication file.

## Configuration

| Variable | Purpose |
| --- | --- |
| `TMM_BACKEND_PATH` | Backend executable override in debug/profile builds; ignored by the release app |
| `TDL_PATH` | Trusted runtime override for debug/profile builds or standalone backend testing; the release app always selects its bundled runtime |
| `TMM_STATE_DIR` | Isolated application state directory for jobs, preferences and caches |
| `XDG_CONFIG_HOME` | Linux configuration-root override |
| `APPLE_SIGNING_IDENTITY` | Optional configured signing identity for macOS packaging |

These variables are configuration, not places to put login codes or session contents. Run synthetic tests with isolated state rather than a personal account.

## Windows and Linux

The source is intended to remain portable. Existing Go CI checks Windows, macOS and Linux; Flutter analysis/widget tests run on Linux. These checks do not validate native account access, file dialogs, helper packaging or real transfer recovery.

Windows/Linux app distribution is experimental. Generate the missing native Flutter host for that platform in a branch and review it. Windows executables need `.exe` names, including `tdl-tmm.exe` and the preview helper; current root Makefile targets use Unix-style names. Graceful pause is disabled on Windows. Platform contributions should include native validation evidence before adding supported download assets.

## Brand assets

The user-supplied original logo is preserved in [docs/assets/logo-source.png](assets/logo-source.png). The current source is 1268×1240 pixels. The shared, transparent [runtime logo](../apps/desktop/assets/branding/logo.png) is a 512×512 square export, fitted with proportional padding; the artwork is not stretched or redrawn. The 1024×1024 native icon is the largest app export. Flutter uses it in the app header and the README displays it at a restrained size.

The macOS icon catalog is `apps/desktop/macos/Runner/Assets.xcassets/AppIcon.appiconset/`, with PNG sizes from 16 to 1024 pixels matching `Contents.json`. Keep those exports consistent with the shared logo and verify small sizes, transparent edges, light/dark appearance and the built app's icon after an asset change.
