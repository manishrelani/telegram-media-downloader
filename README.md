<p align="center">
  <img src="apps/desktop/assets/branding/logo.png" alt="Telegram Media Downloader logo" width="112" height="112">
</p>

# Telegram Media Downloader

Browse Telegram chats and topics, choose the files you want, and download them to your Mac with progress for each file.

[Download for macOS](https://github.com/manishrelani/telegram-media-downloader/releases/download/v0.1.0-alpha.1/Telegram-Media-Downloader-v0.1.0-alpha.1-macos-arm64.dmg) · [Installation](#installation) · [Telegram login](#telegram-login) · [Help](#help)

**Current preview:** [v0.1.0-alpha.1](https://github.com/manishrelani/telegram-media-downloader/releases/tag/v0.1.0-alpha.1). Read the release notes for known limitations before using this early version.

![Telegram Media Downloader showing topics, media selection and download progress](docs/assets/library-preview.png)

*Preview using sample data.*

## Features

- Browse channels, groups and forum topics.
- Select individual files or download a whole chat/topic with your chosen file types.
- Download videos, images, audio, PDFs, Office files and other documents.
- View video thumbnails and track each file in Downloads.
- Search, filter and sort media and download history.
- Retry unfinished work and skip previously saved files that are still present.
- Use light/dark appearance and resize the chat sidebar to fit longer names.

## Requirements

- A Mac with **Apple Silicon** (M1 or later).
- **macOS 13 or later**.
- A Telegram account with access to the chats and media you want to download.
- Free space on your Mac and your chosen download drive.

The macOS installer includes the backend and [tdl](https://github.com/iyear/tdl) runtime for Telegram login and transfers. No source build or separate tdl installation is required. Complete the [Telegram login](#telegram-login) steps before downloading media.

## Installation

1. Open [Releases](https://github.com/manishrelani/telegram-media-downloader/releases) and download `Telegram-Media-Downloader-v0.1.0-alpha.1-macos-arm64.dmg` from the preview release's **Assets**.
2. Open the `.dmg` and drag **Telegram Media Downloader** into **Applications**.
3. Eject the disk image and open the app from Applications.
4. For first-time setup, quit the app and follow [Telegram login](#telegram-login) below.

Choose the `.dmg` app installer. GitHub's **Source code** archives are for developers.

The initial installer is not notarized by Apple. If macOS blocks an app you trust, follow [Apple's instructions to open it through Privacy & Security](https://support.apple.com/en-us/102445).

<details>
<summary>Optional: verify the download</summary>

Download `SHA256SUMS.txt` from the same release. In Terminal, from the folder containing the `.dmg` and checksum file, run:

```bash
shasum -a 256 -c SHA256SUMS.txt
```

A matching download reports `OK`.

</details>

## Telegram login

**Log in once using Terminal.** There is no in-app login screen yet. Later launches reuse the saved session unless it expires or is revoked.

1. Quit Telegram Media Downloader and any other running tdl commands.
2. Open **Terminal** and run:

   ```bash
   "/Applications/Telegram Media Downloader.app/Contents/MacOS/tdl-tmm" login -T qr
   ```

3. On your phone, open Telegram → **Settings → Devices → Link Desktop Device** and scan the QR code shown in Terminal. Complete any further account prompts.
4. Wait for login to finish, then open Telegram Media Downloader and select **Refresh** to load your chats.

If QR login does not work, use phone-number/code login and follow the prompts:

```bash
"/Applications/Telegram Media Downloader.app/Contents/MacOS/tdl-tmm" login -T code
```

These commands assume the app is in `/Applications`. Adjust the path if you installed it elsewhere.

For other login methods, including an existing Telegram Desktop session, follow the official [tdl quick start](https://docs.iyear.me/tdl/getting-started/quick-start/). Where it uses `tdl`, substitute the full bundled executable path shown above and keep the default session. For login or connection errors, see [tdl troubleshooting](https://docs.iyear.me/tdl/more/troubleshooting/).

## Download your first files

1. Select a chat in the sidebar, then a forum topic if applicable.
2. Browse recent media. Use **Load older** for earlier files; search and browser filters apply to loaded results.
3. Choose file types and select the files you want. Use the whole-chat/topic download action when you want every matching file in that chat or topic.
4. Choose a save folder and start the download.
5. Follow each file in Downloads and open its saved location when complete.

Thumbnails load still images, not full videos. Some media has no usable thumbnail. Telegram access restrictions, rate limits and network conditions still apply.

## Updates and local data

Updates are manual: finish or pause active downloads, quit the app, then install the newer version from Releases. Your downloaded files and local app data remain in their existing locations.

Your Telegram session, download history and files are stored locally. The app connects to Telegram using your account; there is no project-owned cloud service. Keep login codes, QR codes and session files private.

## Disconnect or uninstall

To disconnect, quit the app and end the corresponding session in Telegram's **Settings → Devices**. See [Telegram's session guidance](https://telegram.org/faq#q-my-phone-was-stolen-what-do-i-do). Other tdl tools on this Mac may share that session.

To uninstall, quit the app and move it from Applications to the Trash. Removing the app leaves your downloads, local history/cache and tdl session data in place; it does not revoke the Telegram session. See [app data locations](docs/DEPENDENCIES.md#local-data) and the [tdl data guide](https://docs.iyear.me/tdl/more/data/) before removing local data.

## Help

Read [Troubleshooting](docs/TROUBLESHOOTING.md) or [open an issue](https://github.com/manishrelani/telegram-media-downloader/issues/new/choose). Include your app version, macOS version and reproduction steps, with private information removed.

Report app and bundled-tdl problems here first so they can be investigated together. Suspected vulnerabilities should be [reported privately](SECURITY.md).

## Contributing

Bug reports, documentation fixes and code contributions are welcome. See [Contributing](CONTRIBUTING.md) for the workflow and [Development](docs/DEVELOPMENT.md) to build and test the app.

## License and acknowledgements

Licensed under [GNU AGPLv3 (AGPL-3.0-only)](LICENSE). Third-party components retain their own licenses; see [Dependencies and notices](docs/DEPENDENCIES.md).

Built with [tdl](https://github.com/iyear/tdl), [gotd](https://github.com/gotd/td) and [Flutter](https://flutter.dev/). This project is independent of Telegram and tdl. Download only content you are authorized to access and save.
