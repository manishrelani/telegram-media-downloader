# Getting started

[Back to the README](../README.md)

Use a GitHub app release to run Telegram Media Downloader. Developers can use the [source setup guide](DEVELOPMENT.md).

## Download and verify

1. Open [GitHub Releases](https://github.com/manishrelani/telegram-media-downloader/releases).
2. Read the release notes and choose the app ZIP for your Mac's architecture. Current local validation covers **Apple Silicon**. The native host declares macOS 12.0 as its deployment target; the release notes should state the macOS versions actually tested.
3. Download the attached app ZIP and checksum file, when provided. GitHub's **Source code (zip)** download contains source, not an installed app.
4. In Terminal, change to the download folder and run `shasum -a 256 "APP-ARCHIVE.zip"`, substituting the actual filename. Compare the result with the release checksum. A checksum detects a mismatch; trust also depends on where you obtained the archive and checksum.
5. Extract the app ZIP and move the `.app` into Applications. The instructions below assume it is named `Telegram Media Downloader.app`; use its actual name if different.

If no app asset is available yet, [build from source](DEVELOPMENT.md). The presence of a build script or local test result does not mean a release has already been published.

## First launch on macOS

Current local builds are **ad-hoc signed and not notarized**. GitHub distribution does not require the Mac App Store. Developer ID signing and notarization are optional improvements to the direct-download experience.

If macOS cannot verify a downloaded app, first confirm the release source and checksum. For an app you trust, follow Apple's per-app instructions: try opening it, then check **System Settings → Privacy & Security → Open Anyway**, if available. [Apple's first-launch guidance](https://support.apple.com/en-us/102445)

If macOS reports malware, a modified/damaged app, or your organization blocks it, stop and investigate or contact the maintainer. This guide does not require disabling Gatekeeper, removing quarantine recursively, using `sudo` or granting Full Disk Access.

## Connect your Telegram account

The app uses the bundled `tdl-tmm` runtime. First-time login currently happens in Terminal; there is no in-app QR login flow yet.

With the app closed, run:

```bash
"/Applications/Telegram Media Downloader.app/Contents/MacOS/tdl-tmm" version
"/Applications/Telegram Media Downloader.app/Contents/MacOS/tdl-tmm" login -T qr
```

Adjust the path to where you installed the app. Scan the QR prompt using Telegram's **Settings → Devices → Link Desktop Device**, following the prompts for your account. Keep the QR code, login codes and session files private. See the [upstream quick start](https://docs.iyear.me/tdl/getting-started/quick-start/) for account login details.

An existing compatible tdl session may already work. Close other tdl commands while using the app because the local database is shared. Open Telegram Media Downloader and select **Refresh** after login. Use [Troubleshooting](TROUBLESHOOTING.md) if the connection fails.

## Download your first files

1. Select a chat in the sidebar. Expand its forum topics when available, then choose the topic you want.
2. Browse the recent files. Use **Load older** for more results. Search applies to the files already loaded.
3. Select file types and individual files. To fetch a whole scope, use the explicit **Download topic/chat** action and review the selected file types.
4. Choose an existing save folder using the folder picker, then start the download. Reconnect an external drive before selecting its folder.
5. Follow the queue for each file's state and available percentage, bytes, speed or ETA. Use the saved-file actions after completion.

Browsing video previews requests thumbnail images, not a full video download. Thumbnails appear individually as ready; some media has no usable preview.

## Manage downloads

- **Pause/Resume:** where offered, pause requests a graceful interruption and saves recovery information. Wait for **Paused** before quitting. Resume continues the original scope and destination; exact real-file recovery still needs broader validation.
- **Cancel:** stops the selected work while preserving completed files.
- **Retry:** retries unfinished work. A saved file that is missing or fails the saved-file checks can need downloading again.
- **Filters:** combine status, chat/topic and file properties where available. Clear filters if expected files seem absent. Filters do not expand unloaded history.
- **Sidebar:** drag its divider to fit longer names; use the divider's keyboard controls or reset action when needed.

## Files, storage and privacy

The selected destination contains friendly chat folders, optional topic folders, category folders and a hidden download manifest. Captions provide filenames when available; collisions receive suffixes.

```text
Selected folder/
  Chat name [chat ID]/
    Videos/
    Documents/
    Attachments/
    Topic name [topic ID]/          # when downloading a forum topic
    .telegram-media-manager-manifest.json
```

Topic folders have their own category folders. Exact names depend on the chat/topic and media.

The product rename retains the existing state-directory and manifest names for compatibility. The default macOS app state is under `~/Library/Application Support/telegram-media-manager/`. Jobs, preferences, preview cache and private working files can contain sensitive metadata. The Telegram session belongs to tdl's local storage; it is separate from the app's job store.

Allow temporary free space on the app-state disk as well as the final save volume. Avoid deleting state, manifests or recovery folders to resolve an active failure. See [Data handling](DEPENDENCIES.md#local-data) before sharing diagnostics.

## Update or remove the app

Updates are manual through GitHub Releases; no automatic updater is implemented. Pause/finish active work, quit the app, verify the newer release and replace the `.app`. Keep your download folder and app state unless the release notes instruct a documented migration.

Removing the `.app` leaves downloads, app state and the tdl session in their existing locations. Removing saved files or revoking Telegram sessions is a separate user decision. Preserve unfinished recovery data until you have decided whether to resume it.
