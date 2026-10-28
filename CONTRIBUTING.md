# Contributing

Documentation fixes, reproducible bug reports, accessibility improvements, tests and code contributions are welcome.

## Report a bug or suggest a feature

Search [existing issues](https://github.com/manishrelani/telegram-media-downloader/issues) first, then use the issue templates. Include the app version or source commit, operating system, reproduction steps and sanitized error messages. Discuss larger changes in an issue before starting work.

Report app and bundled-tdl problems here first so maintainers can distinguish integration issues from upstream bugs. Report security vulnerabilities [privately](SECURITY.md).

## Make a change

1. Fork the repository and create a branch for your change.
2. Follow [Development](docs/DEVELOPMENT.md) to build and run the app. Automated tests use synthetic data and need no Telegram login.
3. For a behavior change, add a regression test, observe it fail, then implement the fix. Follow existing Go and Dart formatting conventions.
4. Run the relevant checks and update any affected instructions. Documentation changes need link and command verification.
5. Open a pull request explaining the problem, the change and how you tested it. Include screenshots for visual changes when useful.

Keep pull requests focused and discussions respectful. Use synthetic fixtures; never commit Telegram sessions, private messages, downloads, signing keys or personal app state.

## Checks

Run from the repository root:

```bash
make test
make test-flutter
make smoke
```

See [Development](docs/DEVELOPMENT.md#checks) for security scanner setup. Report which checks you ran and any limitations. Transfer changes need attention to storage failures, cancellation, duplicate detection and restart recovery; UI changes should cover keyboard navigation, light/dark themes and compact windows.

Protocol changes should update the [protocol reference](docs/PROTOCOL.md). Dependency updates should follow [Dependencies](docs/DEPENDENCIES.md#updating-dependencies). Maintainers can follow [Releasing](docs/RELEASING.md) when preparing a versioned app.

## Commits and licensing

Use a short, descriptive commit message, for example `fix: preserve saved files after a chat rename` or `docs: clarify Telegram login`. Suggested prefixes are `feat:`, `fix:`, `docs:`, `test:`, `ci:` and `chore:`.

Original contributions use [AGPL-3.0-only](LICENSE). Preserve third-party license notices. Review your public Git author name/email and staged files before publishing.
