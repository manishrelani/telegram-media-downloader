# Media library and topic downloads

The desktop app opens a chat as a media browser. Forum groups expand into topics, including an explicit All topics entry. Loading metadata never starts a media download.

Recent discovery requests the newest 101 attachments and displays 100. Load older increases that window by 100, replacing the snapshot rather than appending duplicate rows. Search and sorting apply to loaded results. The browser labels loaded counts and whether older files remain. File sizes/duration are omitted when upstream metadata is unavailable.

Users may combine Videos, PDFs, Office, Images, Audio, Text, eBooks, Archives, Code, and Other. All files selects every category. Classification is extension-based; ambiguous `.ts` names are treated as source code in this browser. Metadata comes from `tdl chat export --with-content --raw`; this is private local data, not a cloud index.

## Quiet library interface

The desktop interface uses the approved Quiet library design: flat file rows, aligned metadata, warm neutral surfaces, forest-green actions, consistent outline icons, and system typography on macOS. Light/dark appearance follows the operating system. Semantic theme tokens and the approved design rules live in `design-system/telegram-media-downloader/MASTER.md` and `apps/desktop/lib/ui/quiet_theme.dart`.

File types use one multi-select menu with an All files option. Search is explicitly scoped to loaded files. Command/Ctrl+F returns to file search from the queue and reveals the field if needed. Opening/closing the queue preserves the browser's scroll position. Hidden panes are excluded from keyboard focus. The queue expands below the library on tall windows, and uses the full content area on smaller windows.

The bottom transfer strip retains the active filename, last known percentage and available bytes/speed/ETA, with pause/resume and a queue control. It never substitutes invented percentages for missing telemetry. Loading placeholders are static, cached results remain usable, folder errors appear next to the destination, and indicators honor reduced-motion settings. File lists and queues remain virtualized.

## Download scope

Download selected uses immutable snapshot membership and stable attachment keys (`chatId:messageId:originalName`). The backend validates every requested key against both the chat/topic and chosen types, and persists a filtered export before transfer. It never accepts a filename from the UI as a download source.

Download entire topic/chat requires an explicit UI action and confirmation naming the scope and types. It scans the full scope when preparing, then saves that exact item list for recovery. Browser search does not restrict this action. Album expansion is disabled so selecting one attachment cannot implicitly add other messages.

## File queue and progress

Each file has its own saved state: queued, preparing, downloading, finalizing, completed, paused, cancelled or failed. The downloader receives one file per invocation and preserves its per-file staging/input paths. This trades subprocess startup overhead for unambiguous progress attribution and immediate finalization. Discovery can acquire the shared session/database slot between files. Cached pages work while a transfer is active; uncached pages may wait for a long file to finish or be paused.

ANSI terminal styling is stripped before parsing. Events contain the stable item key and observation time. Partial updates merge only within that file; absent telemetry never means the transfer failed. The UI retains the last known percentage, hides stale speed/ETA, and shows the age of the last update. A transfer is marked saved only after the CLI exits successfully, the expected output exists, known byte size matches, the file is organized, and its manifest is saved. Unknown size/percentage remains unknown.

The queue displays every item and supports batch pause/resume/cancel and retry of unfinished work. Saved files expose Open and Show in folder. Retry keeps the original export, topic, types and destination. Completed files are reconciled with the filesystem before being skipped. If the destination is missing, the batch stops with a recoverable storage error.

## Storage compatibility

Chat directories retain the existing `<name> [<id>]` format. Scoped topic outputs use `<chat>/<topic title> [<topic id>]/Videos|Documents|Attachments`. Whole-chat exports use a topic folder when raw message metadata identifies a known topic. Archives/unrecognized attachment extensions use Attachments under the existing organizer rules.

A chat-wide manifest stores optional relative final paths, so a file downloaded through a topic is recognized when selecting it later through All topics. Old manifests without these paths use the prior category/name lookup. Job and manifest version numbers remain compatible: new fields are additive. Old chat jobs retain their original resume path; their discovered file names are shown with Not tracked rather than inventing historical per-file progress.

Temporary raw exports are permission-restricted and deleted after browser ingestion. At most 16 recent snapshots are retained in memory, with a 64 MiB combined budget where possible (a single explicitly requested full-history export can exceed it). Job exports remain private on disk for recovery. `TMM_STATE_DIR` selects an isolated application state directory for testing; it never changes the `tdl` account/session directory.

## Validation and remaining gates

Go unit/race tests, vet, Python entitlement tests, Flutter analysis and 24 Flutter tests pass locally. Render checks cover desktop and compact layouts. The macOS release compiles. `make smoke` covers both the prior workflow and a synthetic subprocess workflow for topics, recent/older discovery, exact selection, partial progress, cross-scope duplicate detection and persisted restart state.

These checks do not use a real Telegram account. Real-topic completeness, General-topic handling, checksum-verified nontrivial Pause/Resume, native folder/file actions and external-volume disconnection still require a live macOS acceptance run. Windows/Linux remain source-compatible targets awaiting native runtime validation; Windows graceful pause remains disabled.

## Previews and discovery filters

Video rows reserve a 16:9 preview column with a duration badge. The raw export can supply a cached JPEG or an embedded stripped Telegram preview; the backend validates and expands these without downloading the video. Unsupported/absent/corrupt previews retain the video icon. Stripped previews are inherently low resolution. The app now bundles a thumbnail-only tdl extension to retrieve actual image sizes in the background; the tiny embedded image is no longer stretched into a cover. Spoiler previews are suppressed, including existing local cache entries. The JPEG format header is attributed to gotd under its MIT license in `docs/third-party/` and the macOS app resources.

The Flutter cache decodes only built rows, retains up to 128 images in memory, limits each preview to 256 KiB and 1280px dimensions, and periodically trims disk storage to 256 entries. Disk cache failures do not block browsing. Cache location is the platform cache directory, or `TMM_STATE_DIR/thumbnails` for isolated runs.

Both library and download views provide status, type, size and posted-date filters with search, independent grouping and sorting, removable filter labels and Clear all. Downloads also offers searchable multi-chat and topic selection. The library keeps its current sidebar scope. Multiple choices within one category are alternatives; different categories intersect. Dates use local calendar days and an inclusive custom end date. Size buckets use decimal MB/GB and put unknown sizes in a separate bucket; unknown sizes sort last in both size directions.

Never-queued attachments are visible only in the library. Downloads filters the app's queued/history records. Not downloaded excludes unverified legacy records and unavailable saved locations. Missing files, failed transfers and unavailable saved locations appear under Needs attention. Refresh downloads checks saved paths; availability is observational and never rewrites a completed job's history.

Selected files survive view changes, with a visible hidden-selection count and Clear selection action. Exact downloads derive types from the actual selection; whole-topic/chat downloads continue to use the explicit scope and type choices, independently of search/status/size/date view filters. Narrow windows use a compact status menu, and only large windows use the expanded download tray. Flat/type/status grouping retains batch actions with explicit batch labels.

## Sharp thumbnail upgrade

The blurry previews were 40×22 embedded placeholders. After the initial page result, the UI requests actual thumbnail images for visible video rows only (preferably the smallest size covering 192×108 physical pixels for a 96×54 row). Document locations retain exact 64-bit IDs from raw JSON and stay inside the backend. `thumbnailVersion` identifies the document version without exposing access hashes/file references to Flutter.

The bundled `tdl-tmm-previews-v2` helper is registered as a local, versioned tdl extension on first use. It reuses the existing login via tdl's extension handoff. It can request only explicit image thumbnail sizes: no empty location (full document), animated preview or video download route is accepted. Each image is limited to 256 KiB; ordered batches are bounded to 12 with four concurrent requests, a network deadline, and a process deadline. The extension SDK/core are pinned at v0.20.4; gotd is pinned at v0.140.0. Helper source/license live in `thumbnail-helper/`.

Network previews use the existing CLI slot at low priority. Starting a transfer or discovery cancels preview work, including the owned extension process group on macOS/Linux. Cached previews are still available during transfers. Persistent server cache lives under `TMM_STATE_DIR/preview-cache-v2` (or the normal application state directory) with a 256-file/64 MiB bound. Flutter accepts `media_thumbnails` only for a known/current snapshot, preserves progress and selections while updating images, and retains the improvement during later job updates.

The UI uses the screen pixel ratio for image decoding and high quality resampling. Images below the logical thumbnail size retain the clean video glyph/duration placeholder until a useful image arrives. Unsupported/unavailable larger previews stay as a glyph, rather than permanently displaying a stretched 40px image. Spoiler suppression applies to both sources. A live, limited check on one cached video confirmed 40×22 → 320×180 with zero video transfers.


## Visible streaming — 2026-10-27

The helper emits one tagged JSONL record as soon as each JPEG is ready. Neither a blocked sibling nor process exit holds completed images back. The backend validates, publishes and caches images individually, retaining successes if another image fails or is cancelled. The v2 helper has a new extension identity; the previous installed v1 helper remains untouched.

A shared Flutter scheduler coalesces visibility for 80ms, deduplicates repeated files and sends one ordered snapshot batch at a time. Actual viewport intersection is measured after layout/paint and on scroll/resize: cached sliver rows, Offstage panes and collapsed groups create no demand. Pending work leaving the viewport is cancelled; completed images remain reusable. Downloads/discovery keep priority, with a two-second busy retry for still-visible files. Errors do not loop; leaving/re-entering or refreshing can retry. Changing chat/topic, replacing a snapshot and disposing the app cancel stale work.

The Quiet library layout and controls are unchanged. No additional filter or progress UI is needed for thumbnails.


## Storage and duplicate reliability — 2026-10-28

Finalization now fails fast with its actual storage cause; earlier saved media and unfinished staging remain available for recovery. Folder creation/listing failures share the same classification. The old batch workflow also reports partial saved/failed outcomes when it stops. Missing/ambiguous output remains a separate failure condition.

A chat's display name is no longer used to choose another folder when its stable ID already has saved output in that destination. Direct-child manifests support discovery after restart even if job history has been removed; no folder migration or filename rewrite occurs. Concurrent first choices are serialized. If prior versions already produced multiple folders for the same chat, the app asks for resolution and does not guess or merge them. Broken identities and symlink chat roots are rejected rather than silently bypassed.

Captions are display/filename metadata, not evidence that media changed. Matching saved files survive caption edits. New manifests record exact raw document/photo IDs, so a known replacement is downloaded even when its original filename and size match; existing files are never overwritten. Missing or size-mismatched saved files are downloaded again. Old manifests without this optional identity remain compatible, with the existing path/size fallback. The app cannot prove old content identity or checksum equivalence when those were never recorded.

Automated coverage includes real filesystem failures during journaling/moves, a complete backend protocol storage-failure/retry cycle, title/caption edits across restarts, exact 64-bit IDs, replacements, legacy manifests, root isolation/ambiguity, missing/truncated files, concurrent folder choices, and legacy resume after rename. Live-account and external-drive acceptance checks remain separate.
