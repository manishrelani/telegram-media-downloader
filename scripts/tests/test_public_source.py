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
 def test_removed_tracked_docs_are_not_in_working_snapshot(self):
  with tempfile.TemporaryDirectory()as temp:
   root=Path(temp);subprocess.run(['git','init','-q',str(root)],check=True)
   (root/'README.md').write_text('public instructions')
   (root/'docs').mkdir();old=root/'docs/STATUS.md';old.write_text('old notes')
   subprocess.run(['git','-C',str(root),'add','.'],check=True)
   old.unlink()
   self.assertEqual(module.candidate_files(root),['README.md'])
 def test_tracked_internal_planning_is_rejected(self):
  for name in ['docs/superpowers/plans/notes.md','docs/IMPLEMENTATION_PLAN.md',
               'docs/PHASE-1-DOMAIN.md','docs/STATUS.md','docs/TEST_MATRIX.md',
               'docs/DOCUMENTATION_RESEARCH.md','design-system/app/MASTER.md']:
   with self.subTest(name=name),tempfile.TemporaryDirectory()as temp:
    root=Path(temp);subprocess.run(['git','init','-q',str(root)],check=True)
    file=root/name;file.parent.mkdir(parents=True,exist_ok=True);file.write_text('local notes')
    subprocess.run(['git','-C',str(root),'add',name],check=True)
    with self.assertRaises(ValueError):module.candidate_files(root)
 def test_deleted_working_copy_does_not_hide_staged_private_files(self):
  for name in ['.env','account.session','local-state/jobs.json','build/generated.txt']:
   with self.subTest(name=name),tempfile.TemporaryDirectory()as temp:
    root=Path(temp);subprocess.run(['git','init','-q',str(root)],check=True)
    file=root/name;file.parent.mkdir(parents=True,exist_ok=True);file.write_text('private fixture')
    subprocess.run(['git','-C',str(root),'add','-f',name],check=True)
    file.unlink()
    with self.assertRaises(ValueError):module.candidate_files(root)
