import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('package_sources', ROOT/'scripts/package_third_party.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class PackageSourcesTests(unittest.TestCase):
    def test_bundles_checksums_after_download_updates_them(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary)
            archive=root/'dependency.zip'
            archive.write_bytes(b'exact upstream source fixture')
            for name in ['thumbnail-helper','runtime/tdl']:
                folder=root/name
                folder.mkdir(parents=True)
                (folder/'go.mod').write_text('module fixture\n')
                (folder/'go.sum').write_text('before download\n')
            def download(command,cwd,**kwargs):
                (cwd/'go.sum').write_text('after download\n')
                info={'Path':'example.com/dependency','Version':'v1.0.0','Zip':str(archive),'Sum':'h1:fixture'}
                return subprocess.CompletedProcess(command,0,json.dumps(info),'')
            with patch.object(module,'ROOT',root),patch.object(module.subprocess,'run',side_effect=download):
                module.package(root/'sources')
            for name in ['thumbnail-helper','runtime/tdl']:
                self.assertEqual((root/'sources'/name/'go.sum').read_bytes(),(root/name/'go.sum').read_bytes())
