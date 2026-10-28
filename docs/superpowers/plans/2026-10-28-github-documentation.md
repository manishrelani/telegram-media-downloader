# GitHub Documentation Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans inline. User explicitly requested research and writing the documentation in this session. No additional approval round for reversible documentation work; the owner subsequently selected AGPLv3 for original code.

**Goal:** Explain direct GitHub downloads, actual app behavior and contributions using patterns researched from established open-source projects.
**Architecture:** Short root README; focused user, dependency, development, troubleshooting and release guides. Preserve historical implementation notes with clear current-document links. No app behavior changes or publishing.
**Tech Stack:** GitHub Markdown, existing Go/Flutter/Python build and verification tools.
**Spec:** User request: GitHub distribution only, no stores; research popular repositories and write the documentation; identify remaining gaps.

## Global constraints
- Keep existing app, builds, account data and prior releases intact.
- Describe the app-owned patched tdl runtime; no global CLI substitution.
- Keep macOS Apple Silicon validation distinct from experimental Windows/Linux sources.
- No invented repository URL, license, security reporting contact, release availability or notarization status.
- Signing/notarization optional for direct distribution; explain Gatekeeper and safe first launch accurately.

## Review focus
- Clean clone commands resolve existing macOS host files and all three executables.
- Release source ZIP differs from runnable app ZIP; no fake download links.
- Support reports avoid account data and do not misroute patched runtime failures upstream.
- Documentation links and bundled-runtime versions match actual source.
- No store, signing or automated-update requirements imposed on the user's chosen distribution.

## Tasks
- [x] Research: HandBrake, LocalSend, OBS Studio, yt-dlp, official GitHub/Apple/tdl docs; record sources and adaptations.
- [x] Write: README, user/developer/dependency/troubleshooting/release guides; contribution templates and screenshot; correct obsolete entry points.
- [x] Verify: local links/images/fences, referenced commands/versions, Python tests, public source policy and secret scan; fresh independent documentation review, resolve important findings, updated source ZIP.

Owner confirmed AGPLv3 (AGPL-3.0-only). The owner created https://github.com/manishrelani/telegram-media-downloader.git. Local documentation now uses that URL. The owner requested reviewing the proposed commit/commands before Git initialization, commits or pushing; no ZIP exports are wanted.
