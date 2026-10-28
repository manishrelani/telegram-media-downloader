# Storage and Duplicate Fixes Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans inline; TDD for each fix. Track evidence in .superpowers/sdd/storage-duplicates/progress.md.

**Goal:** Stop storage failures with actionable causes and prevent metadata edits from re-downloading saved media; deliver the macOS app.
**Architecture:** Fail fast at filesystem boundary; resolve stable chat roots before queueing; compare additive content identities in compatible manifests.
**Tech Stack:** Existing Go backend/Flutter macOS UI, no new dependencies.
**Spec:** docs/superpowers/specs/2026-10-28-storage-duplicates-design.md

## Global constraints
- Keep UI and existing filenames/paths intact; no migrations/deletion/account actions.
- Keep manifest/job schemas backward compatible; no plaintext credentials or access hashes in new identity fields.
- Non-Git supplied workspace: use local sources/ledger, no branches/commits available.

## Review focus
- Partial successful moves must remain recoverable after the next item fails.
- Changed media must not be mistaken for caption-only edits.
- Missing/corrupt files and ambiguous saved chat roots must not be silently accepted.
- Stable IDs must never cross chat/destination boundaries.
- Resume and retry must use original locations and preserve saved files.

## Task 1 — finalization storage errors
- [x] Add organizer and application regressions for typed errors, real manifest-save failure, later files remaining queued, partial success and retry. Observe RED.
- [x] Return recorded summary plus typed storage error at item filesystem failures. Keep missing/ambiguous output distinct. Verify focused then full backend suite.

## Task 2 — duplicate identity and roots
- [x] Add failing tests for edited captions, renamed chats across restart/history deletion, content replacements, missing/wrong-size files, conflicting roots and scope isolation.
- [x] Add exact raw content identity to MediaItem/ManifestEntry, retain legacy compatibility, remove caption equality as completion condition.
- [x] Add application chat-root resolution (recorded jobs/direct-child manifests, stable ID, serialized first choice). Wire both download entry points and preserve resume destination. Verify focused/full backend/race suites.

## Task 3 — integration and macOS delivery
- [x] Extend synthetic protocol smoke for rename/caption/restart/duplicate behavior and finalize-storage error protocol where practical.
- [x] Run make test/smoke and Flutter analyzer/full suite. Obtain one fresh independent review; fix important findings with RED/GREEN.
- [x] Build macOS release, package current backend/helper, verify packaged protocol smokes/signature/ZIP; update status and deliver artifact.
