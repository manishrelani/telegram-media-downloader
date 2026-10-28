"""Package a verified Apple Silicon app as a versioned DMG; never publishes it."""
import argparse
import hashlib
import os
from pathlib import Path
import platform
import plistlib
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
VERSION = r'(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:alpha|beta|rc)\.(?:0|[1-9][0-9]*))?'


def validate_version(tag, root=ROOT):
    """Require vX.Y.Z (or alpha/beta/rc.N) to match Flutter's version exactly."""
    if not re.fullmatch('v' + VERSION, tag):
        raise ValueError('Use a tag such as v0.1.0 or v0.1.0-alpha.1.')
    source = (Path(root) / 'apps/desktop/pubspec.yaml').read_text()
    match = re.search(r'^version:\s*(' + VERSION + r')\+([0-9]+)\s*$', source, re.MULTILINE)
    if not match or match[1] != tag[1:]:
        raise ValueError('The release tag must match the version in apps/desktop/pubspec.yaml.')
    return match[1], match[2]


def macos_version(value):
    if not isinstance(value, str) or not re.fullmatch(r'[0-9]+(?:\.[0-9]+){1,2}', value):
        raise ValueError('Cannot determine the minimum macOS version.')
    parts = tuple(int(part) for part in value.split('.'))
    return parts + (0,) * (3 - len(parts))


def package(tag, root=ROOT):
    root = Path(root)
    version, build = validate_version(tag, root)
    if platform.system() != 'Darwin':
        raise ValueError('Create the macOS disk image on macOS.')
    app = root / 'apps/desktop/build/macos/Build/Products/Release/Telegram Media Downloader.app'
    info_path = app / 'Contents/Info.plist'
    if not info_path.is_file():
        raise ValueError('Build the complete app with scripts/build_macos_app.sh first.')
    with info_path.open('rb') as stream:
        info = plistlib.load(stream)
    native_version = version.split('-', 1)[0]
    if (info.get('CFBundleShortVersionString'), info.get('CFBundleVersion')) != (native_version, build):
        raise ValueError('The app version/build is stale. Rebuild the app for this tag.')
    minimum = macos_version(info.get('LSMinimumSystemVersion'))
    host = info.get('CFBundleExecutable', '')
    if not host or Path(host).name != host:
        raise ValueError('The app has an invalid executable name.')
    executables = [app / 'Contents/MacOS' / name for name in
                   [host, 'telegram-media-downloader-backend', 'tdl-tmm', 'tdl-tmm-previews-v3']]
    for executable in executables:
        if not executable.is_file() or not os.access(executable, os.X_OK):
            raise ValueError(f'Missing app executable: {executable.name}')
    notices = app / 'Contents/Resources/ThirdParty'
    for name in ['tdl-extension-LICENSE', 'gotd-thumbnail-LICENSE', 'go-runtime-LICENSE',
                 'rebuild-sources/sources.json', 'rebuild-sources/REBUILD.txt',
                 'rebuild-sources/runtime/tdl/go.mod', 'rebuild-sources/runtime/tdl/go.sum',
                 'rebuild-sources/thumbnail-helper/go.mod', 'rebuild-sources/thumbnail-helper/go.sum',
                 'thumbnail-helper-source/main.go', 'thumbnail-helper-source/LICENSE']:
        if not (notices / name).is_file():
            raise ValueError(f'Missing bundled source or notice: {name}')
    output = root / 'build/releases' / tag
    if output.exists():
        raise ValueError(f'Release output already exists; review it before rebuilding: {output}')
    subprocess.run(['/usr/bin/codesign', '--verify', '--deep', '--strict', str(app)], check=True)
    for executable in executables:
        subprocess.run(['/usr/bin/lipo', str(executable), '-verify_arch', 'arm64'], check=True)
        metadata = subprocess.run(['/usr/bin/xcrun', 'vtool', '-show-build', str(executable)],
                                  capture_output=True, text=True, check=True)
        versions = re.findall(r'^\s*minos\s+([0-9.]+)\s*$', metadata.stdout, re.MULTILINE)
        if not versions:
            raise ValueError(f'Cannot determine the minimum macOS version of {executable.name}.')
        for required in versions:
            if macos_version(required) > minimum:
                raise ValueError(f'{executable.name} requires macOS {required}, newer than the app target. Rebuild it with MACOSX_DEPLOYMENT_TARGET set.')
    output.mkdir(parents=True)
    image = output / f'Telegram-Media-Downloader-{tag}-macos-arm64.dmg'
    sums = output / 'SHA256SUMS.txt'
    try:
        with tempfile.TemporaryDirectory(prefix='dmg-staging-', dir=output) as temp:
            stage = Path(temp)
            subprocess.run(['/usr/bin/ditto', str(app), str(stage / app.name)], check=True)
            (stage / 'Applications').symlink_to('/Applications')
            subprocess.run(['/usr/bin/hdiutil', 'create', '-volname', 'Telegram Media Downloader',
                            '-srcfolder', str(stage), '-format', 'UDZO', str(image)], check=True)
        subprocess.run(['/usr/bin/hdiutil', 'verify', str(image)], check=True)
        with image.open('rb') as stream:
            checksum = hashlib.sha256()
            for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                checksum.update(chunk)
            digest = checksum.hexdigest()
        sums.write_text(f'{digest}  {image.name}\n')
    except Exception:
        shutil.rmtree(output)
        raise
    return image, sums


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('tag', help='Version tag matching pubspec.yaml, such as v0.1.0')
    parser.add_argument('--check-version', action='store_true', help='Validate the tag without packaging')
    options = parser.parse_args()
    try:
        if options.check_version:
            print('Version matches:', validate_version(options.tag)[0])
        else:
            for asset in package(options.tag):
                print(asset)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f'Release packaging failed: {error}\n')
