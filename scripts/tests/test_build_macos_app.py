"""Run packaging with controlled build tools; repeated builds must preserve notices."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]

class BuildMacOSAppTests(unittest.TestCase):
    def test_repeated_packaging_replaces_read_only_license_files(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root/'scripts').mkdir()
            script=root/'scripts/build_macos_app.sh'
            shutil.copyfile(ROOT/'scripts/build_macos_app.sh',script)
            (root/'apps/desktop/macos').mkdir(parents=True)
            (root/'backend').mkdir()
            (root/'runtime/tdl').mkdir(parents=True)
            (root/'runtime/tdl/go.mod').write_text('fixture')
            (root/'runtime/tdl/go.sum').write_text('fixture')
            (root/'scripts/package_third_party.py').write_text('import pathlib,sys\npathlib.Path(sys.argv[1]).mkdir(parents=True,exist_ok=True)\n')
            (root/'thumbnail-helper').mkdir()
            (root/'docs/third-party').mkdir(parents=True)
            for name in ['tdl-extension-LICENSE','gotd-thumbnail-LICENSE','go-runtime-LICENSE']:
                path=root/'docs/third-party'/name;path.write_text('license notice');path.chmod(0o444)
            for name in ['main.go','go.mod','go.sum','LICENSE']:
                path=root/'thumbnail-helper'/name;path.write_text('helper source');path.chmod(0o444)
            tools=root/'tools';tools.mkdir()
            programs={
                'uname':'#!/bin/sh\necho Darwin\n',
                'go':'''#!/usr/bin/env python3
import os,pathlib,sys
output=pathlib.Path(sys.argv[sys.argv.index('-o')+1]);output.write_text('binary');output.chmod(0o755)
''',
                'flutter':'''#!/usr/bin/env python3
import pathlib
folder=pathlib.Path('build/macos/Build/Products/Release/Telegram Media Downloader.app/Contents/MacOS');folder.mkdir(parents=True,exist_ok=True)
(folder/'tdl-tmm-previews-v2').write_text('obsolete vulnerable helper')
old=folder.parent/'Resources/ThirdParty/rebuild-sources';old.mkdir(parents=True,exist_ok=True);(old/'stale.zip').write_text('old dependency source')
''',
                'codesign':'#!/bin/sh\nexit 0\n',
            }
            for name,body in programs.items():
                path=tools/name;path.write_text(body);path.chmod(0o755)
            env=dict(os.environ,PATH=f'{tools}{os.pathsep}{os.environ["PATH"]}')
            for attempt in range(2):
                result=subprocess.run(['bash',str(script)],env=env,text=True,capture_output=True)
                self.assertEqual(result.returncode,0,f'packaging attempt {attempt+1}: {result.stderr}')
            notices=root/'apps/desktop/build/macos/Build/Products/Release/Telegram Media Downloader.app/Contents/Resources/ThirdParty'
            self.assertTrue((notices.parent.parent/'MacOS/tdl-tmm').is_file())
            self.assertTrue((notices.parent.parent/'MacOS/telegram-media-downloader-backend').is_file())
            self.assertFalse((notices.parent.parent/'MacOS/tdl-tmm-previews-v2').exists())
            self.assertFalse((notices/'rebuild-sources/stale.zip').exists())
            self.assertEqual((notices/'tdl-extension-LICENSE').read_text(),'license notice')
            self.assertEqual((notices/'go-runtime-LICENSE').read_text(),'license notice')
            self.assertEqual((notices/'thumbnail-helper-source/LICENSE').read_text(),'helper source')
