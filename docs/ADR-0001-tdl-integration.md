# ADR-0001: How the desktop backend will integrate with tdl

- **Status:** Provisional; requires live smoke test.
- **Date:** 2026-10-27

## Context

The desktop app needs authenticated chat discovery, media-message export with message text/captions, bulk downloads, readable progress events, and safe process interruption/resume. The `tdl` CLI documents `tdl chat ls -o json`, `tdl chat export -c CHAT --with-content -o PATH`, `tdl dl -f PATH -d DIR`, `--takeout`, `--continue`, and `--template`. The CLI's terminal progress output is dynamic and can use carriage returns.

## Options considered

### A. Managed `tdl` CLI process

The Go backend invokes the user's pinned `tdl` binary with `os/exec`, parses chat JSON, passes paths as individual arguments, captures stderr for diagnostics, and translates recognized progress output into structured JSONL events.

- Pros: low code coupling; easier to pin and update the CLI separately; no direct source dependency; easy to test with a fake executable.
- Cons: progress parsing depends on terminal output; process signals differ by operating system; a dedicated `tdl` installation is needed unless a binary is bundled; subprocess usage does not automatically settle all licensing/distribution questions.

### B. Adapt/import `tdl` Go packages or maintain a fork

The Go package exposes download-related APIs, but its implementation may still render progress or prompt on the terminal. A proper headless adapter could provide typed progress and controlled cancellation.

- Pros: potentially better structured progress and lifecycle control.
- Cons: tighter coupling to implementation details, dependency/version maintenance, and the need to review AGPL-3.0 obligations for distribution.

## Provisional decision

Use the CLI adapter for Phase 0 because it is a small, testable way to validate the overall architecture. The app currently requires `tdl` v0.20.4 or newer for managed downloads: that release's official changelog records a fix for concurrent map read/write while saving interrupted download progress (upstream issue #1327 / PR #1328). This minimum version is a guard against a known lifecycle bug, not proof that Pause/Resume is reliable on every OS or file.

Do not commit to the CLI adapter as the final production integration until a real smoke test proves that the pinned CLI provides adequate progress, error handling, and interruption/resume behavior. If that gate fails, build a second proof using a narrow adapter around the Go packages or a maintained fork.

## Consequences

- Stdout of our Go process is reserved for the versioned JSONL protocol; `tdl` output is captured by the child process.
- Errors from `tdl` are converted to structured error events, with no automatic global process killing.
- Progress parser tests include carriage-return updates and ensure CPU percentages cannot be mistaken for file progress.
- Phase 0 does not declare Pause/Resume release-ready.
