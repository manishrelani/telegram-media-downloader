import importlib.util
from pathlib import Path
import shutil,subprocess,tempfile,unittest,zipfile
ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('public_source',ROOT/'scripts/public_source.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class PublicSourceTests(unittest.TestCase):
 def test_private_and_generated_files_are_excluded_but_lockfile_is_kept(self):
  with tempfile.TemporaryDirectory()as temp:
   root=Path(temp);shutil.copyfile(ROOT/'.gitignore',root/'.gitignore')
   for name in ['.agents/skill.txt','.superpowers/ledger.md','scripts/__pycache__/cache.pyc','.env','account.session','downloads/private.mp4','local-state/jobs.json','build/app','apps/desktop/build/app','apps/desktop/pubspec.lock','backend/go.mod','README.md']:
    file=root/name;file.parent.mkdir(parents=True,exist_ok=True);file.write_text('fixture')
   names=module.candidate_files(root)
   self.assertEqual(set(names),{'.gitignore','apps/desktop/pubspec.lock','backend/go.mod','README.md'})
   archive=root/'build/source.zip';module.build_archive(root,archive)
   with zipfile.ZipFile(archive)as zipped:self.assertEqual(set(zipped.namelist()),{'telegram-media-downloader/'+name for name in names})
 def test_source_snapshot_rejects_symlinks_and_compiled_binaries(self):
  for mode in ['symlink','binary']:
   with self.subTest(mode=mode),tempfile.TemporaryDirectory()as temp:
    root=Path(temp);(root/'backend').mkdir()
    if mode=='symlink':(root/'outside.txt').write_text('private');(root/'backend/main.go').symlink_to(root/'outside.txt')
    else:(root/'backend/manager').write_bytes(b'\x7fELF'+b'\0'*100)
    with self.assertRaises(ValueError):module.candidate_files(root)
 def test_tracked_secret_is_rejected_even_when_ignored(self):
  with tempfile.TemporaryDirectory()as temp:
   root=Path(temp);subprocess.run(['git','init','-q',str(root)],check=True);(root/'.env').write_text('fixture')
   subprocess.run(['git','-C',str(root),'add','-f','.env'],check=True)
   (root/'.gitignore').write_text('.env\n')
   with self.assertRaises(ValueError):module.candidate_files(root)
