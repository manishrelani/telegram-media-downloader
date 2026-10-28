# Publishing and releases

[Back to the README](../README.md)

The intended distribution is **a public GitHub repository plus downloadable GitHub Release assets**. No App Store listing, package registry or hosted service is part of this process.

## Decisions before the first open-source release

- Confirm that the [AGPL-3.0-only project license](../LICENSE) and all tdl/helper and other upstream notices are included in the source and relevant release assets.
- Confirm the [public repository](https://github.com/manishrelani/telegram-media-downloader), maintainer identity and a usable private security-reporting route.
- Replace placeholder app bundle identity/copyright metadata before settling the public package identity. The current macOS host still contains `com.example.telegramMediaManager` and template copyright text in `Runner/Configs/AppInfo.xcconfig`.
- Decide a version/tag and the exact supported OS/architecture. The current locally validated Go bundle is Apple Silicon; do not label it universal based only on the Flutter host.
- Perform real-account video/document downloads and interruption/recovery checks on the intended release Mac. Use an account and content suitable for testing.

The repository can be prepared before all binary acceptance tests are finished. Keep the README and release labels explicit about the current support/preview status.

## Prepare a clean source snapshot

Keep product source, maintained native host files, build scripts, synthetic fixtures, dependency lockfiles and notices in version control. Keep sessions, exports, user downloads, job state, signing keys, caches and release binaries out.

The existing `.gitignore` covers common generated/private files and the publication tool rejects unsafe candidates, including disallowed files already tracked by Git. Review local export/database paths too; an ignore rule is not a substitute for inspecting the staged contents.

```bash
make security
```

Install the scanners using [Development](DEVELOPMENT.md#checks) first. Work directly in the source checkout and review the staged Git diff before committing. No ZIP export is needed for ordinary edits or GitHub pushes. Optional source archives are release assets; build/audit output and previous app packages remain local under ignored `build/`.

## First commit and repository setup

This development folder had no Git history when the publication docs were prepared. Use one clean initial snapshot rather than reconstructing historical commits. Check the Git name/email that will become public first.

After cleaning the sources and running the checks:

```bash
git init -b main
git add --dry-run .
```

Review that listing. Then stage and inspect the actual diff:

```bash
git add .
git diff --cached --stat
git diff --cached --check
git commit -m "chore: publish initial open-source version"
make security
```

The last gate also scans available Git history. If it fails, resolve the finding before a push. Ignore rules do not remove information already committed; a credential exposure needs revocation as well as cleanup.

The public repository is [manishrelani/telegram-media-downloader](https://github.com/manishrelani/telegram-media-downloader). It was reachable with no branches when checked. After reviewing the initial commit and confirming authentication, connect and push:

```bash
git remote add origin https://github.com/manishrelani/telegram-media-downloader.git
git remote -v
git push -u origin main
```

Recheck the remote before the first push in case another commit has been added. Preserve existing remote work; do not force-push an initial import over it. Authentication uses your own GitHub account. Review and authorize a push before publishing; editing the local source does not publish it automatically.

## GitHub settings and Actions

The repository already contains [CI](../.github/workflows/ci.yml) and [Security](../.github/workflows/security.yml) workflows. They cover Go/Flutter checks, secret/dependency gates, helper regressions and macOS Go binary scans. Observe their first successful run on the actual repository before treating them as release evidence.

Enable repository security features available to the account: secret scanning/push protection, dependency alerts and private vulnerability reporting. Protect `main` with required checks. Keep default workflow permissions restricted. Contributor pull requests do not need Telegram sessions or signing secrets.

Dependency-update automation and a tag/manual-triggered draft-release workflow are useful next additions; neither Dependabot nor a full release workflow is currently configured. Release publishing permissions and optional signing secrets should be limited to that release workflow. Do not execute untrusted pull-request code with maintainer secrets.

## Build and choose signing status

```bash
./scripts/build_macos_app.sh
```

Use the complete resulting `.app`, including the three Go executables and dependency sources/notices. Release app startup accepts only the bundled backend/runtime; test/demo fixtures and developer executable overrides are not used. The packaging script removes old preview implementations and stale generated dependency-source archives.

**Direct GitHub distribution does not require an Apple Developer account or App Store submission.** An ad-hoc-signed build can be shared as a clearly labeled preview, with the per-app launch instructions in [Getting started](GETTING_STARTED.md#first-launch-on-macos).

Developer ID signing and notarization are optional improvements to that direct-download experience. The script supports `APPLE_SIGNING_IDENTITY` for a configured signing identity; notarization is not implemented by it. Do not describe an ad-hoc signature as Apple developer verification. Avoid recommending global Gatekeeper overrides.

## Verify the exact app

Before creating a release:

- Run unit/widget/race/static checks and all fake-process smoke tests.
- Validate login, file selection, previews, video/document saving, existing-file behavior, error recovery and restart using the exact bundle.
- Check a real interrupted download against its expected checksum, including an external destination if supported.
- Scan the three bundled Go executables with `govulncheck -mode binary`, then check the source snapshot with `make security`.
- Run `codesign --verify --deep --strict` on the app and verify ZIP integrity after packaging.
- Confirm the included dependency source inventory, notices and module definitions match the release.
- Inspect a clean extraction on the intended architecture; record exactly which macOS version was tested.

See [Development](DEVELOPMENT.md#checks) for scanner versions and [the test matrix](TEST_MATRIX.md) for additional manual cases. Automated checks cannot establish native acceptance by themselves.

## Tag and attach release assets

Create a release from the tested commit/tag, such as `v0.1.0-alpha.1` for an early preview. GitHub Releases associate assets with tags; GitHub also creates source archives automatically. [GitHub release documentation](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases)

Attach a clearly named macOS app ZIP, checksum file, matching project-source snapshot when appropriate and any separate required source/notices. For example, `TelegramMediaDownloader-v0.1.0-alpha.1-macos-arm64.zip` describes the intended version/architecture; it is a suggested naming convention, not an existing download link.

Release notes should state supported OS/architecture, signing/notarization status, bundled tdl/helper versions, changes, known limits and update instructions. Mark an early/incompletely validated binary as a prerelease. A draft gives you a final review point before users are notified.

Keep app ZIPs in Releases rather than Git commits. The README should direct users to the attached app asset; GitHub's **Source code** ZIP is for building. Versioned source assets should remain available with their corresponding binaries.

## Updates and future commits

Updates are manual: users verify the newer release, quit the app and replace its bundle while preserving local state and downloads. Describe any future migration explicitly. No automatic updater is implemented.

For ongoing work, use focused branches and pull requests, run checks and keep commits descriptive. Examples are `fix: preserve resume state on storage failure`, `docs: clarify bundled runtime login` and `ci: add draft release packaging`. Tag releases after verification rather than on every commit.
