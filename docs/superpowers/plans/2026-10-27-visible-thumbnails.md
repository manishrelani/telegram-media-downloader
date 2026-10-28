# Visible Thumbnail Streaming Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans inline. Track RED/GREEN evidence in `.superpowers/sdd/visible-thumbnails/progress.md`.

**Goal:** Make ready thumbnails appear immediately and fetch only currently visible videos.
**Architecture:** Ordered viewport demand → cancellable snapshot request → per-image helper stdout → immediate validated image event and cache.
**Tech Stack:** Flutter/Dart, Go, existing pinned tdl SDK, macOS.
**Spec:** `docs/superpowers/specs/2026-10-27-visible-thumbnails-design.md`

## Global constraints
- No video downloads for previews, no new dependencies or visible controls.
- Preserve 256 KiB/1280px image bounds, spoilers, snapshot membership, transfer priority, selection/progress and existing cache bounds.
- Use `tmm-previews-v2`; do not overwrite the installed v1 helper.
- Work in the supplied non-Git project; no commits/worktree operations are available.

## Review focus
- A blocked/failing sibling must not delay ready images.
- Sliver-cached/offstage rows must not request images.
- Rapid scroll/scope changes must not leak/cross-wire requests.
- Busy/retry, partial success and disposal must not spin or lose cached successes.
- Malformed/duplicate/unrequested helper lines cannot publish images.

## Tasks
1. **Streaming helper** (`thumbnail-helper/main.go`, new stream tests): write blocked-sibling/failure/cancel/concurrency tests, observe RED, implement `streamBatch` and v2 tagged JSONL output, verify helper suite/race/vet.
2. **Streaming backend** (`tdl/thumbnail_fetch.go`, `application/thumbnail.go`, `app/library.go`, associated tests): test per-image delivery before producer exit, key order/membership, malformed/duplicate lines, partial error/cache, busy/cancel. Add ordered `PreviewRequest`, `StreamThumbnails(ctx, requests, emit)` and `request_thumbnails`; remove automatic page prefetch. Verify backend suite/race/vet.
3. **Viewport coordinator** (new Dart scheduler and tests): test ordering/deduplication, 12-key limit, 80ms coalescing, cancellation, retry at 2s, error/re-entry and disposal. Implement a controller-owned scheduler with one active request.
4. **Visibility integration** (new viewport tracker, `video_thumbnail.dart`, both views/controller, widget tests): test offscreen/offstage/scroll/resize, scope changes, streamed selection/progress continuity and no list-wide requests. Implement actual-viewport tracking and connect visible demand to snapshot membership.
5. **Integration/release** (smoke script, Makefile/build script/docs): prove the first event arrives while another image is blocked; prove list-only operations do not fetch. Run full suites/analyzer/smokes, fresh review, repair important findings with RED/GREEN, build and package both helpers/signature/ZIP. Record evidence and deliver the macOS app.
