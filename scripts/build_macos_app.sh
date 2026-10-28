#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$ROOT/apps/desktop"
BACKEND_OUT="$ROOT/build/telegram-media-downloader-backend"
PREVIEW_OUT="$ROOT/build/tdl-tmm-previews-v3"
RUNTIME_OUT="$ROOT/build/tdl-tmm"
export GOTOOLCHAIN=go1.27.3
# Match the Go toolchain's minimum and avoid inheriting the host SDK's newer
# deployment target when cgo links the Telegram runtime.
export MACOSX_DEPLOYMENT_TARGET=13.0
export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=13.0"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=13.0"

if ! command -v go >/dev/null 2>&1; then
  echo "Go (with Go 1.27.3 toolchain support) is required. Install Go and try again." >&2
  exit 1
fi
if ! command -v flutter >/dev/null 2>&1; then
  echo "Flutter 3.35+ is required by this project. Install Flutter and try again." >&2
  exit 1
fi
if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script builds the macOS app and must be run on macOS." >&2
  exit 1
fi

if [[ ! -d "$APP_DIR/macos" ]]; then
  echo "Generating Flutter macOS host folder..."
  (cd "$APP_DIR" && flutter create --platforms=macos .)
fi

mkdir -p "$ROOT/build"
echo "Building Go backend..."
(cd "$ROOT/backend" && go build -trimpath -o "$BACKEND_OUT" ./cmd/manager)
chmod +x "$BACKEND_OUT"
echo "Building patched app-owned Telegram runtime..."
(cd "$ROOT/runtime/tdl" && go build -trimpath -ldflags="-w -X github.com/iyear/tdl/pkg/consts.Version=0.20.4 -X github.com/iyear/tdl/pkg/consts.Commit=security-gotd-0.145.1" -o "$RUNTIME_OUT" github.com/iyear/tdl)
echo "Building thumbnail-only helper..."
(cd "$ROOT/thumbnail-helper" && go build -trimpath -ldflags="-w" -o "$PREVIEW_OUT" .)

echo "Ensuring macOS selected-folder read/write entitlement is present..."
python3 - "$APP_DIR/macos/Runner/DebugProfile.entitlements" "$APP_DIR/macos/Runner/Release.entitlements" <<'PY'
from pathlib import Path
import sys

for raw in sys.argv[1:]:
    path = Path(raw)
    if not path.exists():
        continue
    content = path.read_text(encoding="utf-8")
    key = "com.apple.security.files.user-selected.read-write"
    if key in content:
        continue
    marker = "</dict>"
    if marker not in content:
        raise SystemExit(f"Cannot update entitlement file: {path}")
    block = f"\t<key>{key}</key>\n\t<true/>\n"
    path.write_text(content.replace(marker, block + marker, 1), encoding="utf-8")
PY

echo "Resolving Flutter packages..."
(cd "$APP_DIR" && flutter pub get)
# Apple's native version stays numeric. Prerelease names remain in pubspec,
# release tags and asset names; the build number distinguishes app revisions.
NATIVE_BUILD_NAME="$(python3 - "$APP_DIR/pubspec.yaml" <<'PY'
from pathlib import Path
import re, sys
match = re.search(r'^version:\s*([0-9]+\.[0-9]+\.[0-9]+)(?:-[A-Za-z0-9.-]+)?\+[0-9]+\s*$', Path(sys.argv[1]).read_text(), re.MULTILINE)
if not match:
    raise SystemExit('Expected a version such as 0.1.0+1 in pubspec.yaml')
print(match[1])
PY
)"
echo "Building Flutter macOS release..."
(cd "$APP_DIR" && flutter build macos --release --build-name "$NATIVE_BUILD_NAME")

PRODUCTS_DIR="$APP_DIR/build/macos/Build/Products/Release"
APP_BUNDLE="$PRODUCTS_DIR/Telegram Media Downloader.app"
if [[ ! -d "$APP_BUNDLE" ]]; then
  echo "Build completed but no .app bundle was found in $PRODUCTS_DIR" >&2
  exit 1
fi
BUNDLE_MACOS="$APP_BUNDLE/Contents/MacOS"
if [[ ! -d "$BUNDLE_MACOS" ]]; then
  echo "App bundle has no Contents/MacOS directory: $APP_BUNDLE" >&2
  exit 1
fi
# Flutter may reuse an earlier bundle containing obsolete preview executables.
rm -f "$BUNDLE_MACOS/telegram-media-manager-backend" "$BUNDLE_MACOS/tdl-tmm-previews-v1" "$BUNDLE_MACOS/tdl-tmm-previews-v2"
cp "$BACKEND_OUT" "$BUNDLE_MACOS/telegram-media-downloader-backend"
chmod +x "$BUNDLE_MACOS/telegram-media-downloader-backend"
cp "$PREVIEW_OUT" "$BUNDLE_MACOS/tdl-tmm-previews-v3"
cp "$RUNTIME_OUT" "$BUNDLE_MACOS/tdl-tmm"
chmod +x "$BUNDLE_MACOS/tdl-tmm"
mkdir -p "$APP_BUNDLE/Contents/Resources/ThirdParty"
cp -f "$ROOT/docs/third-party/tdl-extension-LICENSE" "$ROOT/docs/third-party/gotd-thumbnail-LICENSE" "$ROOT/docs/third-party/go-runtime-LICENSE" "$APP_BUNDLE/Contents/Resources/ThirdParty/"

mkdir -p "$APP_BUNDLE/Contents/Resources/ThirdParty/thumbnail-helper-source"
cp -f "$ROOT/thumbnail-helper/"*.go "$ROOT/thumbnail-helper/go.mod" "$ROOT/thumbnail-helper/go.sum" "$ROOT/thumbnail-helper/LICENSE" "$APP_BUNDLE/Contents/Resources/ThirdParty/thumbnail-helper-source/"

# Rebuild generated source notices so removed dependency versions do not linger.
rm -rf "$APP_BUNDLE/Contents/Resources/ThirdParty/rebuild-sources"
python3 "$ROOT/scripts/package_third_party.py" "$APP_BUNDLE/Contents/Resources/ThirdParty/rebuild-sources"

# Copying the helper changes the bundle after Flutter's initial signing step, so sign
# the helper and the final bundle again. Ad-hoc signing does not provide Developer ID verification.
if command -v codesign >/dev/null 2>&1; then
  if [[ -n "${APPLE_SIGNING_IDENTITY:-}" ]]; then
    codesign --force --options runtime --timestamp --sign "$APPLE_SIGNING_IDENTITY" \
      "$BUNDLE_MACOS/telegram-media-downloader-backend"
    codesign --force --options runtime --timestamp --sign "$APPLE_SIGNING_IDENTITY" "$BUNDLE_MACOS/tdl-tmm-previews-v3"
    codesign --force --options runtime --timestamp --sign "$APPLE_SIGNING_IDENTITY" "$BUNDLE_MACOS/tdl-tmm"
    codesign --force --deep --options runtime --timestamp --sign "$APPLE_SIGNING_IDENTITY" "$APP_BUNDLE"
  else
    codesign --force --sign - "$BUNDLE_MACOS/telegram-media-downloader-backend"
    codesign --force --sign - "$BUNDLE_MACOS/tdl-tmm-previews-v3"
    codesign --force --sign - "$BUNDLE_MACOS/tdl-tmm"
    codesign --force --deep --sign - "$APP_BUNDLE"
  fi
fi

echo "\nBuild complete: $APP_BUNDLE"
echo "Direct GitHub builds use ad-hoc signing by default. Developer ID signing and notarization are optional."
echo "Verify first launch, selected-folder access, and child-process access on the release Mac."
