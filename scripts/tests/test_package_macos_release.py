"""Release assets must match the tag and contain a complete, verified app."""
import hashlib
import importlib.util
from pathlib import Path
import plistlib
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location(
    'package_macos_release', ROOT / 'scripts/package_macos_release.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ReleasePackagingTests(unittest.TestCase):
    def setUp(self):
        system = patch.object(module.platform, 'system', return_value='Darwin')
        system.start()
        self.addCleanup(system.stop)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.desktop = self.root / 'apps/desktop'
        self.desktop.mkdir(parents=True)
        (self.desktop / 'pubspec.yaml').write_text('name: fixture\nversion: 0.1.0+1\n')
        self.bundle = self.desktop / 'build/macos/Build/Products/Release/Telegram Media Downloader.app'
        macos = self.bundle / 'Contents/MacOS'
        macos.mkdir(parents=True)
        for name in ['telegram_media_downloader', 'telegram-media-downloader-backend',
                     'tdl-tmm', 'tdl-tmm-previews-v3']:
            executable = macos / name
            executable.write_text('controlled executable')
            executable.chmod(0o755)
        with (self.bundle / 'Contents/Info.plist').open('wb') as stream:
            plistlib.dump({'CFBundleExecutable': 'telegram_media_downloader',
                          'CFBundleShortVersionString': '0.1.0',
                          'CFBundleVersion': '1', 'LSMinimumSystemVersion': '13.0'}, stream)
        self.notices = self.bundle / 'Contents/Resources/ThirdParty'
        for name in ['tdl-extension-LICENSE', 'gotd-thumbnail-LICENSE', 'go-runtime-LICENSE',
                     'rebuild-sources/sources.json', 'rebuild-sources/REBUILD.txt',
                     'rebuild-sources/runtime/tdl/go.mod',
                     'rebuild-sources/runtime/tdl/go.sum',
                     'rebuild-sources/thumbnail-helper/go.mod',
                     'rebuild-sources/thumbnail-helper/go.sum',
                     'thumbnail-helper-source/main.go', 'thumbnail-helper-source/LICENSE']:
            path = self.notices / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('notice or source')

    def test_tag_must_match_source_version_including_prerelease(self):
        self.assertEqual(module.validate_version('v0.1.0', self.root), ('0.1.0', '1'))
        for tag in ['v0.1.1', '0.1.0', 'v01.1.0', 'v0.1.0-alpha.1', '../escape']:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                module.validate_version(tag, self.root)
        (self.desktop / 'pubspec.yaml').write_text('version: 0.1.0-alpha.1+2\n')
        self.assertEqual(module.validate_version('v0.1.0-alpha.1', self.root),
                         ('0.1.0-alpha.1', '2'))

    def test_prerelease_packages_numeric_native_version(self):
        (self.desktop / 'pubspec.yaml').write_text('version: 0.1.0-alpha.1+1\n')
        with patch.object(module.subprocess, 'run', side_effect=self.run_tool):
            image, _sums = module.package('v0.1.0-alpha.1', self.root)
        self.assertIn('v0.1.0-alpha.1', image.name)

    def test_incomplete_bundle_or_sources_are_rejected_before_packaging(self):
        for relative in ['Contents/MacOS/tdl-tmm',
                         'Contents/Resources/ThirdParty/rebuild-sources/sources.json']:
            path = self.bundle / relative
            saved = path.read_bytes()
            path.unlink()
            with self.subTest(path=relative), self.assertRaises(ValueError):
                module.package('v0.1.0', self.root)
            path.write_bytes(saved)
            path.chmod(0o755)

    def test_stale_app_version_or_build_number_is_rejected(self):
        path = self.bundle / 'Contents/Info.plist'
        for version, build in [('0.0.9', '1'), ('0.1.0', '2')]:
            with path.open('wb') as stream:
                plistlib.dump({'CFBundleExecutable': 'telegram_media_downloader',
                              'CFBundleShortVersionString': version,
                              'CFBundleVersion': build}, stream)
            with self.subTest(version=version, build=build), self.assertRaises(ValueError):
                module.package('v0.1.0', self.root)

    @patch.object(module.platform, 'system', return_value='Linux')
    def test_packaging_requires_macos(self, _system):
        with self.assertRaises(ValueError):
            module.package('v0.1.0', self.root)

    def run_tool(self, args, **kwargs):
        name = Path(args[0]).name
        if name == 'ditto':
            shutil.copytree(args[-2], args[-1], symlinks=True)
        elif name == 'hdiutil' and args[1] == 'create':
            stage = Path(args[args.index('-srcfolder') + 1])
            self.assertTrue((stage / 'Telegram Media Downloader.app/Contents/MacOS/tdl-tmm').is_file())
            self.assertEqual((stage / 'Applications').readlink(), Path('/Applications'))
            Path(args[-1]).write_bytes(b'controlled disk image')
        return subprocess.CompletedProcess(args, 0, stdout='    minos 13.0\n')

    @patch.object(module.platform, 'system', return_value='Darwin')
    def test_child_requiring_newer_macos_is_rejected(self, _system):
        def newer_runtime(args, **kwargs):
            if 'vtool' in args and Path(args[-1]).name == 'tdl-tmm':
                return subprocess.CompletedProcess(args, 0, stdout='    minos 27.0\n')
            return self.run_tool(args, **kwargs)
        with patch.object(module.subprocess, 'run', side_effect=newer_runtime):
            with self.assertRaisesRegex(ValueError, 'requires macOS 27.0'):
                module.package('v0.1.0', self.root)
        self.assertFalse((self.root / 'build/releases/v0.1.0').exists())

    @patch.object(module.platform, 'system', return_value='Darwin')
    def test_missing_macos_compatibility_metadata_is_rejected(self, _system):
        def missing_metadata(args, **kwargs):
            if 'vtool' in args:
                return subprocess.CompletedProcess(args, 0, stdout='no build information')
            return self.run_tool(args, **kwargs)
        with patch.object(module.subprocess, 'run', side_effect=missing_metadata):
            with self.assertRaisesRegex(ValueError, 'minimum macOS version'):
                module.package('v0.1.0', self.root)

    @patch.object(module.platform, 'system', return_value='Darwin')
    def test_dmg_checksum_and_bundle_are_preserved(self, _system):
        with patch.object(module.subprocess, 'run', side_effect=self.run_tool) as run:
            image, sums = module.package('v0.1.0', self.root)
        self.assertEqual(image.name, 'Telegram-Media-Downloader-v0.1.0-macos-arm64.dmg')
        digest = hashlib.sha256(image.read_bytes()).hexdigest()
        self.assertEqual(sums.read_text(), f'{digest}  {image.name}\n')
        self.assertTrue((self.bundle / 'Contents/MacOS/tdl-tmm').is_file())
        commands = [call.args[0] for call in run.call_args_list]
        self.assertTrue(any(Path(cmd[0]).name == 'codesign' and '--verify' in cmd for cmd in commands))
        self.assertEqual(sum(Path(cmd[0]).name == 'lipo' for cmd in commands), 4)
        self.assertTrue(any(Path(cmd[0]).name == 'hdiutil' and cmd[1] == 'verify' for cmd in commands))
        with self.assertRaises(ValueError):
            module.package('v0.1.0', self.root)

    @patch.object(module.platform, 'system', return_value='Darwin')
    def test_signature_architecture_and_image_failures_do_not_leave_assets(self, _system):
        for failed_tool in ['codesign', 'lipo', 'hdiutil']:
            def fail(args, **kwargs):
                if Path(args[0]).name == failed_tool:
                    raise subprocess.CalledProcessError(1, args)
                return self.run_tool(args, **kwargs)
            with self.subTest(tool=failed_tool), patch.object(module.subprocess, 'run', side_effect=fail):
                with self.assertRaises(subprocess.CalledProcessError):
                    module.package('v0.1.0', self.root)
            self.assertFalse((self.root / 'build/releases/v0.1.0/SHA256SUMS.txt').exists())
            self.assertFalse(list((self.root / 'build/releases').rglob('*.dmg')))
