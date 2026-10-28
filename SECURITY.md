# Security

This application handles Telegram account access, private message metadata and downloaded files. Keep the Telegram session, app state, working folders and download manifests private. Public source code is expected; account data and signing credentials are not.

## Report a vulnerability

Use GitHub's **Security → Report a vulnerability** on [this repository](https://github.com/manishrelani/telegram-media-downloader/security) when private reporting has been enabled. Until a private channel is configured, do not put credentials, session files, private chat content or working exploits in a public issue. Maintainers must enable private vulnerability reporting before inviting security reports.

## Local checks and publication

Use Go 1.27.3 and the committed Go/Pub lockfiles. Install the pinned scanners:

```sh
export GOTOOLCHAIN=go1.27.3
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
go install github.com/zricethezav/gitleaks/v8@v8.30.1
export PATH="$(go env GOPATH)/bin:$PATH"
make test
make test-flutter
make security
make public-source
```

The security check rejects unsafe publication paths, scans the exact source snapshot with Gitleaks, scans Git history when present, checks imported packages in all three Go targets and queries OSV for every hosted package in the Flutter lockfile. Missing tools, network failures and incomplete responses fail the check. Scan reports are local files under ignored `build/`.

Publish the inspected source snapshot, not the entire development directory. `.gitignore` excludes local agent tools, exports, sessions, generated builds, signing keys and private state; publication validation also rejects these files if already tracked. Ignore rules cannot undo an earlier exposure: revoke any exposed credential and remove it from history before publishing. No repository history existed in the development folder at the time of hardening.

## Runtime boundary

Packaged apps use their own patched `tdl-tmm` runtime and versioned thumbnail helper. They do not silently fall back to a global CLI. Release app startup ignores executable overrides (`TDL_PATH` and `TMM_BACKEND_PATH`) and requires its bundled executables. In debug/profile builds and standalone backend testing, `TDL_PATH` remains an explicit developer override: setting it means trusting that executable and its dependencies. The bundled runtime is tdl 0.20.4 rebuilt with gotd 0.145.1 and other pinned dependency fixes, using Go 1.27.3. It shares the existing tdl account/session location; packaging does not read or distribute that data.

Downloads run in app-owned private working directories. Final output and manifests use root-confined operations, reject symlinks and preserve exclusive file creation. Legacy recovery data is copied without following links. Saved-file paths are validated before the operating system opens them. A process with control of the same user account can still modify app state or race an external OS file opener; this app is not an isolation boundary against that user.

## Before a public release

Enable GitHub secret scanning and push protection, private vulnerability reporting and dependency alerts. Require passing CI on the default branch. These are repository settings and are not enabled by source files alone.

The macOS build is ad-hoc signed unless a Developer ID is configured. Direct GitHub distribution can use a clearly labeled preview build. Developer ID signing and Apple notarization are optional improvements to the download experience; real-account/real-download validation remains a release acceptance check. See [Releasing](docs/RELEASING.md) and [First launch](docs/GETTING_STARTED.md#first-launch-on-macos). App Sandbox is disabled for this child-process architecture; do not claim Mac App Store sandbox isolation. Dependency sources and license notices ship with the app. The project's original code uses [AGPL-3.0-only](LICENSE); dependencies retain their own license terms and notices.

Security scans describe known issues at scan time and do not guarantee the absence of vulnerabilities. A module-level advisory for the unmaintained `golang.org/x/crypto/openpgp` package may be reported even though this app does not import that package. Imported-package scans remain enforced without an advisory allowlist.
