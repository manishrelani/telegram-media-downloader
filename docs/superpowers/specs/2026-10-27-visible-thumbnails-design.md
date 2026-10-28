# Visible, progressive video thumbnails

Approved request: display each thumbnail as soon as ready, prioritize videos currently visible on screen, preserve the clean UI, use TDD, and deliver the updated macOS app.

The current helper waits for every image in a page batch and returns one file. The backend starts that batch automatically after listing media. Replace both barriers: the viewport requests only visible attachments and a versioned v2 helper streams validated images individually.

Use the existing Flutter/Go/tdl stack and cache, with no new dependencies or visual controls. A viewport tracker excludes offstage views and sliver cache rows. A shared coordinator coalesces visibility changes for 80ms, deduplicates attachments, orders by screen position, limits a request to 12 keys, and cancels requests containing no-longer-visible work. Successful images remain cached. A busy download slot is retried after 2 seconds while the images remain visible; failures do not loop until refresh/re-entry. At most one network preview request runs at once per controller.

The backend accepts snapshot-bound `request_thumbnails` requests; unknown keys, spoilers, oversized lists and unrelated snapshots cannot fetch images. Cached images are emitted first. A v2 helper streams each completed still JPEG via stdout with at most four workers; cancellation retains images already published. New transfers continue to preempt preview work. The existing v1 helper remains untouched.

Alternatives considered: smaller automatic page batches still request offscreen media; per-video helper processes repeat connection startup. A small ordered visible batch with per-image streaming avoids both problems.

Acceptance: a fast image is visible before a deliberately blocked sibling finishes; offscreen rows cause no fetch; scrolling/scope changes cancel stale work; duplicates, partial failures, busy slots, malformed output, cached images and disposal are covered. Filters, selection, progress and image quality remain intact. Verify the full suites and signed local macOS bundle/ZIP.
