# Contributing

Thank you for helping improve Telegram Media Downloader. Useful contributions include documentation, reproducible bug reports, tests, accessibility fixes and native platform validation as well as new features.

The project's original code and original contributions use **AGPL-3.0-only**; see [LICENSE](LICENSE). Keep existing upstream license notices intact. Dependencies retain their own licenses. See [Dependencies](docs/DEPENDENCIES.md).

## Before you start

Read the [README](README.md), [development guide](docs/DEVELOPMENT.md) and [known limitations](README.md#status-and-limits). Search existing issues and pull requests. Discuss larger features before investing in an implementation. Keep reports and reviews respectful, specific and focused on the software.

For suspected vulnerabilities, use [SECURITY.md](SECURITY.md). Public issue templates are for ordinary bugs and feature proposals.

## Set up and choose work

Fork the repository, clone your fork and create a branch for the change. Follow [Development](docs/DEVELOPMENT.md) for the exact Go/Flutter setup and bundled-runtime build. Automated tests use synthetic fixtures and do not require a Telegram login.

Good first contributions include improving a confusing instruction, a small regression test, keyboard/focus behavior, or documenting a reproducible platform issue. Maintainers decide labels and priorities when issues are triaged.

## Test-driven changes

For a behavior change or bug fix:

1. Add a focused regression or contract test and observe the relevant failure.
2. Implement the smallest change that passes it.
3. Refactor with the test still passing.
4. Run checks appropriate to the affected components. Add race coverage for concurrency/lifecycle changes.
5. Update user or developer documentation when behavior changes.

Documentation-only changes need link, command and factual verification rather than artificial tests. Use synthetic fixtures; never add personal chats, session databases or downloaded private media to a test case.

## Checks

Run from the checkout root:

```bash
make test
make test-flutter
make smoke
```

The [development guide](docs/DEVELOPMENT.md#checks) explains each target and scanner installation for `make security`. Before requesting review, report the checks you ran and any that remain unavailable. Do not describe real Telegram downloads, native UI tests or another operating system as tested unless you actually checked them there.

For UI changes, include sample-data screenshots where useful and exercise light/dark appearance, compact windows, larger text, keyboard navigation and loading/error states. For transfer changes, consider interrupted jobs, duplicate detection, storage failure, cancellation and restart recovery.

## Open a pull request

Keep the change focused. Explain the user-visible problem, the resulting behavior and how it was verified. Link the relevant issue, document limitations and include a regression test for a behavioral fix. Review the diff for unrelated generated files and private information.

Use existing Flutter/Go conventions, format the source and preserve the separation between UI, application logic and adapters. Protocol changes need corresponding models, tests and [protocol documentation](docs/PROTOCOL.md).

Changes to tdl dependencies or helper registration need the checks described in [Updating dependencies](docs/DEPENDENCIES.md#updating-dependencies). An upstream version number alone is not proof that the security fixes in our build remain present.

## Commits and merging

Use short, descriptive commits for one coherent change. Suggested prefixes are `feat:`, `fix:`, `docs:`, `test:`, `ci:` and `chore:`:

```text
fix: keep a completed file after a chat title changes
feat: add a keyboard shortcut for the queue
docs: clarify first-time Telegram login
```

Prefer plain messages explaining the result. Do not include credentials or private content in commit messages. Review your Git name/email before publishing; GitHub provides a noreply email option if you prefer it.

A pull request may be asked to change scope, add evidence or address compatibility. Maintainers can squash a related set of commits when merging. CI must pass before a release is cut; publishing a repository does not establish that its configured workflows have already run successfully.

## Reports and community support

Use the issue forms for bugs and feature requests. Include release/commit, platform, reproduction steps and sanitized diagnostics. For a runtime problem, start here so maintainers can distinguish packaging/integration issues from an upstream tdl bug. Avoid duplicate upstream reports; link existing reports where relevant.

Support and reviews are community work, with no guaranteed response time. Help others only within what you can verify, and keep access to other people's private data out of the troubleshooting process.
