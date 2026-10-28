# Releasing a macOS version

Maintainer guide. Users should follow [installation and login](../README.md#installation).

## Version numbers

Use tags such as `v0.1.0`, `v0.1.1` and `v0.2.0`. For previews, use `v0.1.0-alpha.1`, `v0.1.0-beta.1` or `v0.1.0-rc.1`.

The first preview uses `version: 0.1.0-alpha.1+2` in `apps/desktop/pubspec.yaml` and the tag `v0.1.0-alpha.1`. A tag must match the source version excluding the `+` build number. Increment the build number for a new app build.

The build script passes a numeric native app version (for example `0.2.0`) to Flutter. Prerelease suffixes remain in the source version, tag and download filename; the native build number distinguishes preview builds.

## Before creating a tag

1. Update the app version, user-facing changelog and any changed instructions. Commit reviewed changes and confirm CI/Security checks pass for that commit.
2. Build the complete app with `./scripts/build_macos_app.sh` on Apple Silicon.
3. Test Telegram login, topic/file selection, previews, a real video/document download, saved-file detection, interruption/recovery and app restart. Test external-drive behavior if supporting it. Compare a resumed file with its expected checksum.
4. Confirm app identity/copyright metadata, supported macOS versions, included licenses and corresponding dependency sources. The existing bundle identifier is retained for compatibility; changing it requires checking existing app preferences.
5. Keep early builds marked as prereleases until their acceptance checks are complete. Never present automated checks as real-account validation.

## Create a tagged draft release

After review and commit, create an annotated tag on the tested commit and push that tag. For example:

```bash
git tag -a v0.1.0-alpha.1 -m "Telegram Media Downloader v0.1.0-alpha.1"
git push origin v0.1.0-alpha.1
```

Use the version you prepared; these commands are not part of ordinary development. Do not move an already published release tag.

The [macOS release workflow](../.github/workflows/release.yml) runs only on pushed `v*` tags. It validates the version, runs tests/security checks, builds on an Apple Silicon macOS runner, checks the bundled Go binaries, verifies signing and architecture, then creates a **draft** release containing:

- `Telegram-Media-Downloader-v0.1.0-alpha.1-macos-arm64.dmg` (using the actual tag).
- `SHA256SUMS.txt` for download verification.

GitHub supplies source archives for the tagged commit. The app also includes the pinned tdl/helper dependency sources and notices. The draft is not available for public download until a maintainer publishes it.

The workflow uses the repository's `GITHUB_TOKEN`; no Telegram login or personal token is needed. Repository policy must allow the release job's `contents: write` permission. The workflow deliberately leaves published assets untouched; do not overwrite a published version to repair a failed run. A retry after a draft already exists needs maintainer review of that draft first.

## Review and publish

Open the draft under [GitHub Releases](https://github.com/manishrelani/telegram-media-downloader/releases). Verify the downloaded DMG/checksum, install a clean copy, and check the exact release app on the macOS versions you intend to support.

Edit the notes with meaningful changes, known issues, tested OS versions and signing status. The automated build is **ad-hoc signed and not notarized**; it does not use signing secrets. The `v0.1.0-alpha.1` tag automatically creates a prerelease draft. Keep it marked as a prerelease when publishing. Publish only after the release acceptance checks pass.

Keep the README download link, release status and changelog aligned with the published version. For previews, link to that specific release or the releases list; GitHub's `/releases/latest` points to a stable release, not a prerelease.

GitHub's [release guide](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository) explains draft publishing and asset management.

## Package locally

To prepare the same assets after building the complete app:

```bash
python3 scripts/package_macos_release.py v0.1.0-alpha.1 --check-version
python3 scripts/package_macos_release.py v0.1.0-alpha.1
```

Output stays in ignored `build/releases/v0.1.0-alpha.1/`. Packaging creates no tag, commit, upload or release. It rejects mismatched versions, missing binaries/notices, invalid signatures, executables lacking arm64 support and binaries requiring a newer macOS version than the app declares. It does not overwrite an existing version's output. Review or move old local output before rebuilding.

Developer ID signing can be configured for local builds through `APPLE_SIGNING_IDENTITY`; notarization requires a separate process. Record the actual status in release notes and use [Apple's launch guidance](https://support.apple.com/en-us/102445) for user support.
