# Troubleshooting

[Back to the README](../README.md) · [tdl integration](DEPENDENCIES.md)

Start with the symptom below. Use this repository's issue form if it remains reproducible. Share only sanitized diagnostics; session files and private exports are not needed in a public report.

## macOS will not open the app

Confirm that the archive came from the repository's release and matches its published checksum. Read the release's signing status. Current local builds are ad-hoc signed, without notarization.

For an app you trust, follow Apple's [per-app first-launch instructions](https://support.apple.com/en-us/102445). Managed Macs may have additional restrictions. A damaged/modified or malware warning needs investigation. Do not disable Gatekeeper globally, run the app as root or grant Full Disk Access to get past a launch failure.

## Backend or Telegram runtime is missing

Use the complete app ZIP and preserve its bundle contents. Re-extract or rebuild the app rather than copying individual executables from unrelated versions.

For source development:

```bash
make build build-runtime build-thumbnail-helper
```

All three Go executables belong beside each other in `build/`. Check `TMM_BACKEND_PATH` and any explicit `TDL_PATH` override. The default configuration requires the patched sibling runtime, not a global CLI.

For `Operation not permitted` after generating/changing the macOS host, run `python3 scripts/configure_macos_entitlements.py` from the checkout root and rebuild. The maintained app uses a direct-distribution process architecture; regenerating default sandbox entitlements can prevent the backend from starting.

## Login fails or chats do not appear

Close the app and other tdl commands. Check the bundled runtime and authenticate if needed:

```bash
"/Applications/Telegram Media Downloader.app/Contents/MacOS/tdl-tmm" version
"/Applications/Telegram Media Downloader.app/Contents/MacOS/tdl-tmm" login -T qr
```

Adjust the installation path. Reopen the app and select **Refresh**. Confirm network access and that your account can see the chat in Telegram. QR/login/session details are private. The app currently has no built-in login screen.

See [tdl's quick start](https://docs.iyear.me/tdl/getting-started/quick-start/) and [troubleshooting](https://docs.iyear.me/tdl/more/troubleshooting/) for runtime-specific explanations. If using an explicit developer override, report its exact version and dependency build.

## Database is used by another process

Close other app instances and tdl commands normally, then retry. The app serializes its own Telegram operations, but an independent process can still hold tdl's shared database. Preserve session/database files; deleting them is not a normal lock-recovery step.

## A chat, topic or file appears missing

Check account access, the selected topic and active filters. Search covers loaded results; use **Load older** to load additional media. Select **All file types** when a filename has an unusual extension. Explicit whole-topic/chat actions can discover more than the current loaded browser page.

If topic discovery is incomplete, include a small synthetic description of the layout and exact app/runtime versions in a report. Avoid exposing private names or message exports.

## Percentage is absent or progress appears stuck

The runtime does not always emit a usable percentage. The queue keeps the file identity and can show partial byte/speed/state information without inventing progress. Discovery, session waits and final copying can also take time.

Wait for a terminal file state before treating the output as complete. If reproducible, report the visible state, whether bytes change, the file type and a sanitized error. Telegram flood waits and network failures can delay transfers.

## Thumbnails are slow, absent or soft

Visible videos are prioritized and successful images appear individually. Some media only has a small thumbnail or no usable preview. Uncached previews may wait while a transfer holds the shared session slot.

The preview workflow requests still images, not whole videos. A missing preview should not be treated as proof that a video transfer failed. Report whether all previews fail or only specific media, plus the runtime/helper versions.

## Storage error or disconnected drive

Reconnect the original volume, check available space and write permission, then retry unfinished work. Private staging and final output can use different volumes, so check both. Do not create a directory at a missing external-drive mount path as a substitute for reconnecting it.

The app stops subsequent work for storage failures and preserves completed output/recovery data where possible. Symlinks in output or legacy recovery paths are rejected. Use a normal, directly accessible save folder. Do not delete a manifest or staging directory while diagnosing an interrupted job.

## A file downloads again, or a replacement is skipped

Saved-file checks use recorded attachment identity, manifest data, path presence and known size. Missing or wrong-size output can need downloading again; a known replacement should be treated as new media even when its filename/size matches. Older exports may lack a stable content ID.

Report whether the original file exists, whether the caption or actual attachment changed, and whether the destination/chat was renamed. Preserve existing output and describe the issue with synthetic filenames.

## Pause/Resume is unavailable or fails

Graceful pause is currently disabled on Windows. Where available, wait for **Paused** before quitting. Resume and retry use the original destination; reconnect it first. Real-file interruption, restart recovery and checksum matching still need broader platform validation.

Preserve unfinished work and provide the observed state/error. Automated fake-process coverage does not establish reliability for every Telegram attachment or native filesystem.

## What to include in a bug report

Provide the app release or source commit, OS/architecture, bundled runtime version, exact steps, expected/actual results and sanitized error text. For build issues, include Go/Flutter versions and the failing command. Crop/redact screenshots before attaching them.

Report app and patched-runtime integration issues here first. Maintainers can compare with [existing upstream tdl issues](https://github.com/iyear/tdl/issues) and help isolate an upstream reproducer. Suspected vulnerabilities follow [SECURITY.md](../SECURITY.md), not public issue templates.
