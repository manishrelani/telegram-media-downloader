from __future__ import annotations

import importlib.util
import plistlib
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[1] / "configure_macos_entitlements.py"
SPEC = importlib.util.spec_from_file_location("configure_macos_entitlements", SCRIPT)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class ConfigureEntitlementsTests(unittest.TestCase):
    def test_disables_sandbox_and_preserves_existing_keys(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "DebugProfile.entitlements"
            path.write_bytes(plistlib.dumps({
                "com.apple.security.app-sandbox": True,
                "com.apple.security.network.server": True,
                "custom.keep": "value",
            }))

            changed = MODULE.configure_entitlement(path)
            data = plistlib.loads(path.read_bytes())

            self.assertTrue(changed)
            self.assertIs(data["com.apple.security.app-sandbox"], False)
            self.assertIs(data["com.apple.security.files.user-selected.read-write"], True)
            self.assertIs(data["com.apple.security.network.server"], True)
            self.assertEqual(data["custom.keep"], "value")

    def test_is_idempotent(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "Release.entitlements"
            path.write_bytes(plistlib.dumps({
                "com.apple.security.app-sandbox": False,
                "com.apple.security.files.user-selected.read-write": True,
            }))

            self.assertFalse(MODULE.configure_entitlement(path))
            self.assertFalse(MODULE.configure_entitlement(path))

    def test_rejects_non_dictionary_plist(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "bad.entitlements"
            path.write_bytes(plistlib.dumps(["unexpected", "array"]))
            with self.assertRaises(ValueError):
                MODULE.configure_entitlement(path)


if __name__ == "__main__":
    unittest.main()
