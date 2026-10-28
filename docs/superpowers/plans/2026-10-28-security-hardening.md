# Security Hardening Implementation Plan
> **For agentic workers:** Use superpowers:executing-plans inline and TDD. Ledger .superpowers/sdd/security-hardening/progress.md.
**Goal:** Fix all three findings, prepare public sources, add checks and rescan deliverables.
**Architecture:** Root-confined finalization, validated saved paths, patched app-owned runtime/helper; exact publication snapshot and security CI.
**Tech Stack:** Go1.27.3, Flutter, pinned tdl0.20.4 with patched dependencies, Python.
**Spec:** docs/superpowers/specs/2026-10-28-security-hardening-design.md
## Constraints
Preserve downloads/session/global CLI; no publish; no design changes; no deletion of developer folders. Non-Git workspace: local ledger/artifacts.
## Review focus
Output symlinks/races; forged manifest/Open paths; legacy/topic/retry compatibility; stale helper/runtime selection; clean publication without scanner exceptions.
## Tasks
1. [x] Filesystem/manifest/Open regressions RED; implement confinement/validation, backend suite/race.
2. [x] Malicious MTProto-length regression; pin patched runtime dependencies/toolchain, app-owned tdl/new helper identity; build/scan.
3. [x] Publication/security automation tests RED; ignores/lockfile/security policy/pinned CI; exact source archive.
4. [x] Full suites/smokes, fresh review and TDD fixes, signed local app build, rescan exact deliverables and report.
