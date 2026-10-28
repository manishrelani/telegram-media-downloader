# Layer 3 — Download orchestration implementation note

## What is implemented

The `internal/application.Service` coordinates a one-chat download without Flutter or Telegram dependencies leaking into the domain layer:

1. Validate request inputs and ensure the destination root already exists as an absolute directory.
2. Reject duplicate job IDs and concurrent jobs for the same chat.
3. Persist `queued` and `preparing` job states.
4. Export message text/captions via the `tdl` adapter.
5. Parse exported media rows and apply the selected extension-based filter.
6. Load the per-chat manifest and skip a completed item only if its saved file still exists and its caption matches.
7. Filter the original export JSON to pending media rows while preserving unknown upstream fields.
8. Download into a job-specific hidden staging directory.
9. Organize staged files into `Videos`, `Documents`, and `Attachments`, using the message caption as the filename and preserving the source extension.
10. Save completed file mappings and the terminal job state, then emit structured events.

The persistent job store and per-chat manifest use write-temp, sync, and rename patterns with restrictive local permissions. Jobs are stored under the platform's user config directory, and the chat manifest contains local file metadata only. Telegram session secrets are not copied into these stores.

## TDD proof points

- Domain tests cover state transition validity, filters, safe caption filenames, collision suffixes, manifest identity and progress summaries.
- JSON store tests cover restart loading, deterministic listing, invalid jobs, corrupt JSON, cancellation and file permissions.
- Manifest store tests cover missing-file initialization, round trip, chat ID mismatch, malformed JSON, unsupported versions and private permissions.
- Organizer tests cover video/PDF/other categories, empty captions, duplicate titles, missing/ambiguous staging matches and refusing to overwrite.
- Application service tests cover end-to-end staging/rename/persist, duplicate names, manifest skipping toggle, recovery after a final file is deleted, crash-window reconciliation, filters, export failure, empty filters, cancellation, missing destination and same-chat concurrency.
- App handler tests cover the `start_chat_download` protocol action and queue listing.
- `scripts/smoke.sh` runs Go tests, race detector, vet, build and a fake-`tdl` protocol workflow that checks the final caption-derived filename.

## Known limitations and next work

- There is no batch request yet: the API starts one chat/job per request.
- The organizer now writes a `moving` manifest record with the planned final filename before each file move. If the app crashes after the move but before the `completed` update, a resume checks that final path, promotes the entry to completed and does not download that item again. A test simulates exactly this window. External modification/corruption of the final file is not detected with a checksum yet.
- Disk-space estimates/monitoring, transient retry with bounded backoff, detailed per-item persisted states and total byte counts remain open.
- The `tdl` CLI process lifecycle remains provisional. In particular, cancellation must be differentiated from pause, and continuation must be tested against the exact supported `tdl` version before enabling Pause/Resume in a UI.
- No real Telegram account/binary was available in this environment. The fake smoke test validates the app orchestration contract, not Telegram compatibility.
