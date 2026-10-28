# Backend JSON Lines protocol v1

The Flutter app starts `telegram-media-downloader-backend` as a child process and communicates via UTF-8 JSON Lines: one request object per stdin line, one event/result object per stdout line. Standard output is protocol-only. Startup diagnostics and captured `tdl` diagnostics must not be printed into protocol output.

Every request contains `version: 1`, a unique non-empty `id`, an `action`, and optional `params`. Events echo the request ID. For `start_chat_download`, the request ID is also the persistent job ID and must be unique across the job store.

## Actions

| Action | Params | Output |
|---|---|---|
| `health` | none | `result` with executable version, `managedDownloadsSupported`, `minimumSafeManagedDownloadVersion`, and `supportsGracefulPause`; does not itself prove account authentication |
| `list_chats` | none | `result.data.chats` |
| `list_media` | `chat`, optional `topic: {id,title}`, `limit` (100 for recent, larger for older, 0 explicitly all), optional `refresh` | asynchronous `result` with `items`, `snapshotId`, `hasMore`, `cached`, `loadedAt`; control requests remain responsive |
| `cancel_media_request` | `requestId` | Cancels obsolete metadata discovery without cancelling downloads |
| `start_media_download` | `chat`, `destinationRoot`, `selection`, optional `takeout` | `job_accepted`, persisted `job_state` including `files`, stable-item `download_progress`, terminal job event |
| `retry_download` | `jobId` of failed/cancelled library job | Reuses the same persisted job, file selection, export and original destination; completed outputs are reconciled/skipped |
| `open_download_file` | `jobId`, `itemKey`, optional `reveal` | Opens/reveals only an existing completed file recorded in that job |
| `export_chat` | `{ "chat": "CHAT_ID_OR_USERNAME", "outputPath": "..." }` | `result` or `error` |
| `download_export` | `exportPath`, `directory`, optional `filter`, `include`, `takeout`, `continue`, `group`, `skipSame` | low-level progress/completion events; mainly for diagnostics |
| `start_chat_download` | `chat: { id, type, name, username? }`, `destinationRoot`, optional `filter`, `takeout`, `group`, `skipSame` | `job_accepted`, repeated `job_state`, `download_progress`, then `job_completed` or `job_failed`/`job_cancelled` |
| `list_jobs` | none | `result.data.jobs` from persistent state |
| `pause_download` | `{ "jobId": "ACTIVE_JOB_ID" }` | Requests graceful interruption only when the backend reports support; the job later emits `job_paused` or an error |
| `resume_download` | `{ "jobId": "PAUSED_JOB_ID" }` | `job_accepted`, then lifecycle/progress events using the persisted job identity |
| `cancel_download` | `{ "jobId": "ACTIVE_OR_PAUSED_JOB_ID" }` | Requests cancellation of active work or cancels an inactive paused/queued job |

`filter` is one of `videos_and_documents` (default), `videos_only`, `documents_only`, `all_attachments`.

## Topic/file selections

Chat JSON includes optional `topics: [{id,title}]`. Topic IDs are separate from chat IDs. All topics is represented by an absent `topic`, never a guessed topic ID.

```json
{"version":1,"id":"browse-1","action":"list_media","params":{"chat":{"id":123,"name":"Course","type":"group"},"topic":{"id":42,"title":"Backend"},"limit":100}}
```

The result includes up to 100 attachments, before UI type/search filtering. Increasing the limit returns a replacement window of recent attachments. A snapshot ID identifies the exact exported metadata; it is held in a bounded in-memory cache. Expired snapshots require a refresh/reselection rather than silently broadening a request.

```json
{"version":1,"id":"files-1","action":"start_media_download","params":{"chat":{"id":123,"name":"Course","type":"group"},"destinationRoot":"/Volumes/Data","selection":{"topic":{"id":42,"title":"Backend"},"fileTypes":["pdf","office"],"snapshotId":"SNAPSHOT_FROM_BROWSE","keys":["123:7:notes.pdf"]}}}
```

`fileTypes` is a nonempty set of `video`, `pdf`, `office`, `text`, `ebook`, `archive`, `code`, `other`, or `all`. Unknown values fail closed. `selection.entireScope: true` explicitly replaces snapshot/keys with a full scan of that chat/topic, filtered by the same file types. The app disables automatic album expansion for this workflow.

Library jobs have `library: true`, `destinationRoot`, optional `topic`, `fileTypes`, `selection`, `items` and `files`. Each file carries `item`, `state`, optional `progress`, `savedPath`, and `error`. File states include queued/preparing/downloading/finalizing/completed/paused/cancelled/failed. Progress has `jobId`, `itemKey`, full `fileName`, `updatedAt` (Unix milliseconds), and any known bytes/percentage/speed/ETA fields. Missing fields are partial observations, not zeros or failures. Never merge different item keys.

Successful process exit alone does not complete a library file; the output must be found and finalized. Job completion requires every requested file to be completed, including verified already-saved files. Old job/store formats remain readable; legacy chat jobs keep their original lifecycle and cannot supply historical per-file telemetry.

All session/database operations use the shared `tdl` slot. Cached browsing can respond while a file transfers. Uncached discovery waits for a slot between transfers, and its request can be cancelled while waiting.

## Start a chat download

```json
{"version":1,"id":"job-unique-001","action":"start_chat_download","params":{"chat":{"id":4488394154,"type":"channel","name":"ChaiCode GenAI"},"destinationRoot":"/Volumes/Data/TelegramDownloads","filter":"videos_and_documents","takeout":true}}
```

The destination root must already exist and be an absolute directory path. This avoids accidentally creating a mount-point directory on the internal drive when an external disk has been disconnected.

Events are framed as:

```json
{"version":1,"id":"job-unique-001","event":"job_state","data":{"job":{"id":"job-unique-001","state":"downloading"}}}
{"version":1,"id":"job-unique-001","event":"download_progress","data":{"progress":{"jobId":"job-unique-001","percent":35.1,"speedBytesPerSecond":1970000,"etaSeconds":92}}}
{"version":1,"id":"job-unique-001","event":"job_completed","data":{"job":{"id":"job-unique-001","state":"completed"},"summary":{"renamed":3,"skipped":0,"failed":0,"alreadyCompleted":0,"outcomes":[]}}}
```

The snippets above are structural examples, not literal output for a real chat. The actual summary includes per-file outcomes. The Go service uses a global single-slot guard around `tdl` work because all jobs share its local session/database state; additional jobs remain queued rather than starting parallel `tdl` processes.

## Pause, resume and cancellation

Managed downloads require a stable `tdl` release v0.20.4 or newer. An older or unrecognized version causes start/resume/download requests to return `tdl_version_unsupported`. The `health` action re-reads the version so an upgrade can be recognized after Refresh.

`pause_download` is accepted only when `health.data.supportsGracefulPause` is true. That flag requires both the minimum `tdl` version and a platform with graceful process interruption. On supported Unix platforms, the Go supervisor sends an interrupt to the child process it created, waits for exit, and the application persists the same job as paused while retaining the filtered export and staging directory. `resume_download` reuses that job and requests `tdl --continue`. `cancel_download` is separate: it requests cancellation and preserves completed files. The mechanics are covered by fake-adapter tests, but the actual checkpoint/continue behavior must still be smoke-tested with the supported `tdl` release on each target OS before advertising Pause/Resume as release-guaranteed. Windows currently does not advertise graceful pause.

## Errors

Errors use a structured shape:

```json
{"version":1,"id":"request-id","event":"error","error":{"code":"invalid_params","message":"destination root must be an existing directory"}}
```

Potential codes include `unsupported_protocol_version`, `invalid_request`, `invalid_params`, `unavailable`, `unauthorized`, `rate_limited`, `storage_error`, `download_failed`, `start_download_failed`, `job_not_found`, and `unknown_action`. Consumers should display the `message` while using the stable `code` for UI handling.

## Additive preview and availability metadata

`MediaItem.thumbnail` is an optional base64-encoded validated JPEG extracted from cached/stripped thumbnails in the existing raw export. It is absent when unavailable or invalid. `hasSpoiler: true` prohibits showing a preview, including a cached image. These fields do not authorize a separate video download. Image/audio file type selectors are now `image` and `audio`; existing selectors remain accepted.

`list_jobs` enriches completed file tasks with observational `availability`: `available`, `missing`, `unavailable` (for example a disconnected destination), or `unknown` for a legacy record without a saved path. This does not change persisted transfer state. Calling `list_jobs` again refreshes these observations. View filtering/grouping/sorting runs in Flutter against loaded metadata and history; it does not change the immutable selection contract.

## Visible thumbnail requests and progressive events

`list_media` returns metadata and embedded placeholders without fetching page-wide previews. The UI sends an explicit, cancellable request for 1–12 visible video keys from an existing snapshot:

```json
{"version":1,"id":"thumb-request","action":"request_thumbnails","params":{"snapshotId":"immutable-snapshot","keys":["123:7:video.mp4"]}}
```

Each ready image emits immediately, before the remaining images or helper process finish:

```json
{"version":1,"id":"thumb-request","event":"media_thumbnails","data":{"snapshotId":"immutable-snapshot","thumbnails":{"123:7:video.mp4":"BASE64_JPEG"}}}
{"version":1,"id":"thumb-request","event":"result","data":{"busy":false,"cancelled":false}}
```

All keys are checked for snapshot membership before any cache/network work. Duplicate keys are coalesced; spoilers, documents and missing image locations are skipped. The server emits only requested, validated JPEGs. `busy: true` means the CLI slot is occupied or a transfer preempted preview work; the UI can retry after two seconds while those files remain visible. `cancel_media_request` with `requestId` cancels the batch, and its final `result` settles the pending UI request. Ordinary preview failures settle with `thumbnail_unavailable`, preserve successful images and do not create download/global errors.

Document access hashes and file references remain inside the backend. `MediaItem.thumbnailVersion` distinguishes document versions and prevents an old job event from reverting a sharpened preview. Consumers preserve selection, file state and progress while replacing an image, and ignore unrelated snapshots. Preview requests fetch only explicit thumbnail sizes, never videos.


## Storage failures and duplicate identity — 2026-10-28

Filesystem failures during organizer setup, destination inspection, manifest journaling or the final move are classified as `storage_error`. Finalization stops immediately; library jobs retain completed files, mark the affected file failed and leave later files queued. Retry preserves the original destination and reconciles saved files. Legacy batch terminal events retain partial organization outcomes even when finalization fails. Missing or ambiguous upstream output remains distinct from storage errors.

`MediaItem.contentId` and manifest-entry `contentId` are additive optional strings such as `document:9223372036854775000` or `photo:8223372036854775001`. They preserve exact Telegram media IDs without access hashes or file references and are independent of captions/thumbnails. Both export paths request raw metadata. A known content-ID change or mismatched saved size forces another download with collision-safe naming. Legacy manifests without content IDs continue using attachment identity, saved path and available size; their previous content version cannot be retroactively verified.

New batches reuse a saved chat directory by stable chat ID within the selected destination, even after chat-title edits or removal of job history. Existing folders/files are not renamed. Multiple recorded folders for the same chat are rejected with an explicit resolution message rather than merged automatically. Different chats and destination roots remain independent; continuation uses the original recorded path.
