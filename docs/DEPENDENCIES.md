# Dependencies and tdl integration

[Back to the README](../README.md)

## Responsibility

| Component | Role |
| --- | --- |
| Flutter desktop app | Browsing, selection, filters, queue controls and rendering |
| Local Go backend | Scheduling, progress events, filesystem organization, recovery and manifests |
| `tdl-tmm` | App-owned rebuild of tdl for Telegram login, discovery, export and transfer |
| `tdl-tmm-previews-v3` | App-owned tdl extension that requests explicit still-image thumbnails |

The upstream [tdl project](https://github.com/iyear/tdl) is independently maintained. Telegram Media Downloader adds a desktop workflow and maintains its own integration and pinned build recipe. It is not an official Telegram or tdl client distribution.

## Pinned runtime

The current source builds tdl **0.20.4** with patched dependencies, including gotd **0.145.1**, using Go **1.27.3**. The thumbnail helper uses protocol/extension identity **tmm-previews-v3**. x/net is pinned at **0.62.0** in the current module graph.

The authoritative definitions are [runtime/tdl/go.mod](../runtime/tdl/go.mod), [thumbnail-helper/go.mod](../thumbnail-helper/go.mod) and their `go.sum` files. The build script supplies the recognized tdl version string; the version number alone does not establish that dependency security fixes are present.

Packaged apps use their bundled runtime. Backend and Flutter startup do not silently switch to a global tdl executable. `TDL_PATH` is an explicit override for debug/profile app builds or standalone backend testing and means trusting the supplied executable. The release app ignores both `TDL_PATH` and `TMM_BACKEND_PATH`, requires its bundled executables and does not fall back to a development build. Normal users should update through this project's release process.

Build locally with `make build-runtime build-thumbnail-helper`. See [Development](DEVELOPMENT.md) for the full app setup.

## Upstream documentation and support

- [tdl repository and README](https://github.com/iyear/tdl)
- [tdl quick start](https://docs.iyear.me/tdl/getting-started/quick-start/)
- [tdl troubleshooting](https://docs.iyear.me/tdl/more/troubleshooting/)
- [tdl issues](https://github.com/iyear/tdl/issues)
- [gotd repository](https://github.com/gotd/td)

Start with this repository's [troubleshooting guide](TROUBLESHOOTING.md). App UI, progress routing, queue, saved paths, packaging and patched-runtime integration belong here. Maintainers should isolate a reproducible upstream problem before directing someone to the tdl tracker. Include exact runtime/build details: a result from a custom dependency build is not automatically evidence of the same bug in an unmodified upstream release.

Telegram access restrictions, service availability and flood-wait limits can affect the runtime; this project does not bypass them.

## Thumbnails

The helper requests explicit thumbnail locations through the existing tdl session. It has no full-video download route. Requests are bounded; ready JPEGs stream individually and missing/invalid previews are skipped. A local helper registration may be created on first use. See the [helper guide](../thumbnail-helper/README.md).

## Local data

The app stores job history, preferences, preview caches and private work data below its state directory. Defaults are:

On macOS: `~/Library/Application Support/telegram-media-manager/`.

The legacy `telegram-media-manager` directory names and hidden `.telegram-media-manager-manifest.json` filename are intentionally retained so the product rename preserves existing jobs, preferences, caches and duplicate detection. The existing bundle identifier and window-restoration keys are also retained for compatibility. Runtime/helper executable names and `TMM_` environment variables also keep their existing technical identifiers. The repository name is `telegram-media-downloader`.

`TMM_STATE_DIR` overrides the app state root for isolated development. Downloads and hidden manifests live in the selected save folder. tdl manages its own session/database separately; consult its documentation for the location used by your configuration.

Never publish session databases, QR/login codes, private exports, job stores, access hashes, file references or unreviewed logs. A private working directory protects normal transfer handoffs; it is not isolation from a process controlling the same user account.

## Licensing and corresponding source

The tdl runtime and preview helper include **AGPLv3** code. gotd carries an **MIT** license; the Go runtime and Flutter dependencies have their own notices. The project's original code is licensed under [AGPL-3.0-only](../LICENSE). Upstream license terms and notices are preserved.

The macOS packaging script includes notices under `Contents/Resources/ThirdParty/`. Its `rebuild-sources/` directory contains exact dependency source archives, module definitions, checksums and rebuild instructions. The helper source is also included. The Flutter build supplies its dependency notices. The complete release's notices should be reviewed before distribution.

A GitHub source archive of this repository contains the pinned build recipes and app/helper sources; it does not vendor every dependency. When publishing a binary, retain the corresponding source archives/notices in the app and attach the matching project source or tag. See [tdl's license](https://github.com/iyear/tdl/blob/v0.20.4/LICENSE) and [Releasing](RELEASING.md).

## Updating dependencies

Update the relevant module/lockfile on a branch, preserve the helper protocol contract and run the unit/race/vet, smoke and security gates. Rebuild all affected executables and scan the actual release binaries. If an installed helper implementation changes incompatibly, review whether its identity must change. An upstream version bump should not silently remove a pinned security fix.
