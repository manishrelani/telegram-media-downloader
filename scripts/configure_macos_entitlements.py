#!/usr/bin/env python3
"""Configure the macOS host for this app's external Go/tdl process architecture.

This application is intended for local/off-Mac-App-Store distribution. It launches a Go
backend and an independently installed `tdl` executable; the default Flutter App Sandbox
blocks executing programs outside the app bundle/container. App Store builds need a
sandbox-aware architecture instead of this configuration.
"""
from __future__ import annotations

import plistlib
import sys
from pathlib import Path
from typing import Any

SANDBOX_KEY = "com.apple.security.app-sandbox"
SELECTED_FOLDER_KEY = "com.apple.security.files.user-selected.read-write"


def configure_entitlement(path: Path) -> bool:
    """Disable App Sandbox while preserving other keys, and enable selected-folder access.

    Returns True when the file was changed.
    """
    with path.open("rb") as stream:
        payload: Any = plistlib.load(stream)
    if not isinstance(payload, dict):
        raise ValueError(f"Entitlements plist must contain a dictionary: {path}")

    changed = payload.get(SANDBOX_KEY) is not False
    payload[SANDBOX_KEY] = False
    if payload.get(SELECTED_FOLDER_KEY) is not True:
        payload[SELECTED_FOLDER_KEY] = True
        changed = True

    if changed:
        path.write_bytes(plistlib.dumps(payload, fmt=plistlib.FMT_XML, sort_keys=False))
    return changed


def entitlement_paths(root: Path) -> list[Path]:
    runner = root / "apps" / "desktop" / "macos" / "Runner"
    names = (
        "DebugProfile.entitlements",
        "Release.entitlements",
        "Runner-DebugProfile.entitlements",
        "Runner-Release.entitlements",
    )
    return [runner / name for name in names if (runner / name).is_file()]


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    paths = entitlement_paths(root)
    if not paths:
        print(
            "No Flutter macOS entitlement files were found. Generate the macOS host first with "
            "`cd apps/desktop && flutter create --platforms=macos .`, then rerun this script.",
            file=sys.stderr,
        )
        return 2

    for path in paths:
        changed = configure_entitlement(path)
        state = "updated" if changed else "already configured"
        print(f"{state}: {path.relative_to(root)} (App Sandbox disabled)")

    print(
        "This configuration is for local/off-Mac-App-Store builds. "
        "A Mac App Store release requires a sandbox-aware helper architecture."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
