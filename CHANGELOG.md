# Changelog

## Unreleased — Sharp thumbnails

- Replace stretched 40px placeholders with real Telegram thumbnail images loaded progressively through the existing login.
- Bundle a versioned thumbnail-only helper with strict location, byte, concurrency and time limits; preserve exact document IDs.
- Cache sharper images, retain them across job updates, preserve selection/progress, and decode at the screen pixel ratio.
- Show the quiet video glyph while a usable image is unavailable; respect spoilers for all preview sources.
- Prioritize transfers over background image work and stop owned preview process groups cleanly.
- Verify a live 40×22 → 320×180 upgrade without downloading the video.

## Unreleased — Video previews and useful filters

- Show validated Telegram embedded video previews with duration badges, bounded local caching and stable fallback geometry; suppress spoiler previews even when cached.
- Add per-file status, chat/topic, type, decimal size and posted-date filters, with independent grouping and sorting in a clean adaptive toolbar.
- Add Images and Audio categories across Flutter and Go.
- Preserve and disclose hidden selections; derive exact-download types from selected files rather than display filters.
- Distinguish missing files, unavailable locations and unknown legacy status without rewriting download history.
- Add draft filter dialogs, Escape cancellation, Clear all, refreshed saved-file checks and batch controls in flat/grouped lists.
- Validate through failing-first unit/widget tests, expanded protocol smoke checks and light/dark previews.

## Unreleased — Resizable chat sidebar

- Add a draggable chat divider with a 248px default and 220–420px bounds, hover/focus feedback, and double-click reset.
- Remember the chosen width across launches in local UI preferences; preserve it when smaller windows temporarily constrain the sidebar to keep the file browser usable.
- Support keyboard resizing, accessible adjustment actions, and a secondary-click width menu.
- Cover drag limits, reset, persistence, compact layouts, late preference reads, save failure, and alternatives to dragging.

## Unreleased — Quiet library polish

- Apply the approved UI/UX Pro Max design direction to the Flutter desktop app, with semantic forest/neutral themes and system light/dark appearance.
- Replace file cards/filter-chip collections with flat aligned rows and a compact multi-select type menu.
- Add a count-specific download action, contextual folder validation, clearer loading/empty/error states and full-name tooltips.
- Preserve file-browser context while inspecting downloads; adapt the tray to available window space and exclude hidden panes from focus.
- Add Command/Ctrl+F, accessible progress values, readable metadata, focus styles and reduced-motion activity indicators.
- Give the macOS app a friendly display name and remembered window size/position.

## Unreleased — Topic media library

- Add recent/older media browsing, forum topics, search, sorting, exact file selection and explicit full-scope downloads.
- Add selectable video/PDF/Office/text/eBook/archive/code/other filters and All files.
- Persist file-level states and progress; merge partial telemetry by stable identity and recognize ANSI-rendered progress.
- Transfer one file at a time, finalize it immediately, expose saved-file actions and retry unfinished work without changing scope/destination.
- Share CLI database coordination across discovery, downloads and diagnostic operations; preserve cached browsing during transfers.
- Keep old jobs/manifests readable and retain their legacy resume path.
- Add isolated cross-platform smoke state via `TMM_STATE_DIR`; validate topic selections and restart persistence with a compiled-backend smoke test.
- Pass Flutter analysis/tests and macOS release compilation; retain explicit live-account validation gates.

## Unreleased — Phase 6 hardening

- Require a recognized stable `tdl` version v0.20.4 or newer before app-managed download operations.
- Reject pre-release versions even if their numeric version is higher than the supported floor.
- Report the detected `tdl` version and managed-download/Pause capability through the backend health response.
- Re-check `tdl` version on health refresh so upgrading the CLI can re-enable download actions without restarting the backend.
- Improve the Flutter message and disable the start-download control when the installed `tdl` version is not supported.
- Add regressions for prerelease version rejection, unsupported-version job refusal, and two chats with the same display name writing to separate chat-ID folders.
- Clarify that bounded retry behavior is implemented, while free-space preflight and reliable media byte totals remain open.


## Unreleased — macOS backend launch fix

- Diagnose macOS `Operation not permitted` when Flutter attempts to launch the Go backend, including likely App Sandbox cause and remediation.
- Add an idempotent script to disable App Sandbox for the local/off-Mac-App-Store architecture that launches the Go backend and external `tdl` executable.
- Integrate entitlement configuration into the macOS build script and document the Mac App Store architecture limitation.
- Add Python tests for entitlement mutation and Dart tests for actionable process-launch error messages.
