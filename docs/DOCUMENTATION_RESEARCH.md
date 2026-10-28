# Documentation research

Reviewed 28 October 2026. This document records the primary examples used for the repository's documentation structure. It adapts organization and usability patterns; it does not copy their prose, claim affiliation or borrow their support/platform guarantees.

## Examples reviewed

| Project | Pattern observed | Adaptation here |
| --- | --- | --- |
| [HandBrake](https://github.com/HandBrake/HandBrake) | A short introduction followed by clear documentation, support and contribution routes | Keep the root README readable; move setup and troubleshooting into focused guides |
| [LocalSend](https://github.com/localsend/localsend) | Screenshot/download navigation, platform compatibility, toolchain guidance and contributing/build sections | Show a sample-data app preview, separate app download from source build, and state actual architecture/platform status |
| [OBS Studio](https://github.com/obsproject/obs-studio) | Quick links separate user help, builds, development and bug reports; contribution guidance is explicit | Provide direct paths for users and contributors and explain issue routing |
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Release assets distinguish architecture and source; checksums, dependencies and bundled component licenses are documented | Explain which ZIP runs the app, name architecture/signing status and document the patched tdl dependency boundary |

Each project is larger and more mature than this one. Multiple stores, donation links, translation systems and extensive CLI reference sections would add unnecessary surface here, so they were not adopted.

## Primary platform references

- [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases): tags, downloadable assets and automatically generated source archives.
- [GitHub README guidance](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-readmes): explain what the project does, how to start and where to get help; keep longer documentation separate.
- [Apple first-launch guidance](https://support.apple.com/en-us/102445): macOS trust checks and the per-app Open Anyway process for an app the user trusts.
- [tdl repository](https://github.com/iyear/tdl), [quick start](https://docs.iyear.me/tdl/getting-started/quick-start/) and [troubleshooting](https://docs.iyear.me/tdl/more/troubleshooting/): upstream behavior and support references.
- [tdl 0.20.4 license](https://github.com/iyear/tdl/blob/v0.20.4/LICENSE): upstream license reference for the pinned runtime.

## Decisions for this project

Use repository Markdown and GitHub Releases, without a separate documentation website or store workflow. Direct download is the user's chosen distribution route. Signing/notarization can improve that route but is not a prerequisite for sharing an honestly labeled preview.

Keep installation, first login and updates in the user guide; keep SDK/compiler requirements in the development guide. Refer users to the bundled patched runtime and route app/integration issues through this project before claiming an upstream bug.

Use repository-relative links between documents and the confirmed [public repository](https://github.com/manishrelani/telegram-media-downloader) for repository, issue and release navigation. Avoid release badges, download URLs or claims about published assets until they exist. Use a sample-data image, not a personal Telegram screenshot. The maintainer selected AGPLv3 (AGPL-3.0-only) for original project code. Keep remaining maintainer-contact, app-identity and real-device acceptance work visible in the release checklist.
