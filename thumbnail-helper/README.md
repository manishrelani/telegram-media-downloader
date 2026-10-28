# Thumbnail helper

An app-owned tdl extension that retrieves still-image thumbnails through the existing tdl session. It accepts explicit thumbnail locations, streams validated JPEG records individually and has no full-video download route.

## Build and test

From the repository root:

```bash
make build-thumbnail-helper
make test-thumbnail-helper
make test-thumbnail-helper-race
```

The Makefile selects **Go 1.27.3** and builds `build/tdl-tmm-previews-v3` with symbols retained for accurate binary scanning. Dependencies are pinned in [go.mod](go.mod) and [go.sum](go.sum), including gotd **0.145.1** and tdl extension/core **0.20.4**.

The macOS packaging script includes this helper beside the backend and patched runtime. On first use, the app registers the bundled local executable with tdl as **tmm-previews-v3**. It does not download a third-party extension binary at runtime. Review the helper identity when an implementation or protocol change could make an existing registration incompatible.

## Protocol

Input is an ordered array of up to 12 `{key, reference}` records in a private temporary JSON file. An empty `thumbSize` is rejected. Four workers request still-image thumbnails; each successful response is serialized as `{protocol: "tmm-previews-v3", key, image: BASE64_JPEG}`. Failures skip the affected image and cancellation stops pending work.

The host validates bounds, requested identity and JPEG contents again. Thumbnails are scheduled for visible videos and yielded individually; network preview failure does not become a full-video transfer.

The tdl extension SDK uses AGPLv3; see [LICENSE](LICENSE). gotd's MIT notice and other dependency sources/notices are retained in packaged app resources. See [Dependencies](../docs/DEPENDENCIES.md), [Development](../docs/DEVELOPMENT.md) and [Security](../SECURITY.md).
