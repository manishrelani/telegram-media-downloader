"""Create an exact, inspectable public-source snapshot; never publishes anything."""
import argparse
from pathlib import Path, PurePosixPath
import shutil
import stat
import subprocess
import tempfile
import zipfile

TOP_LEVEL = {'.github', '.gitignore', '.gitattributes', '.gitleaks.toml', 'README.md',
             'CHANGELOG.md', 'CONTRIBUTING.md', 'SECURITY.md', 'LICENSE', 'NOTICE',
             'Makefile', 'backend', 'thumbnail-helper', 'runtime', 'apps', 'scripts',
             'docs', 'testdata', 'design-system'}
PRIVATE_PARTS = {'.agents', '.superpowers', '__pycache__', '.git', '.dart_tool',
                 'build', 'Pods', 'ephemeral', '.tdl', '.tdl-work', 'downloads', 'local-state'}
PRIVATE_SUFFIXES = {'.pyc', '.pyo', '.session', '.key', '.pem', '.p12', '.pfx', '.db', '.sqlite'}
BINARY_MAGIC = {b'\x7fELF', b'\xcf\xfa\xed\xfe', b'\xce\xfa\xed\xfe',
                b'\xfe\xed\xfa\xcf', b'\xca\xfe\xba\xbe'}

def candidate_files(root):
    root = Path(root).resolve()
    real = subprocess.run(['git', '-C', str(root), 'rev-parse', '--show-toplevel'],
                          capture_output=True, text=True)
    if real.returncode == 0 and Path(real.stdout.strip()).resolve() == root:
        command = ['git', '-C', str(root), 'ls-files', '--cached', '--others', '--exclude-standard', '-z']
        result = subprocess.run(command, capture_output=True, check=True)
    else:
        with tempfile.TemporaryDirectory(prefix='tmm-public-index-') as temp:
            repository = Path(temp) / 'index.git'
            subprocess.run(['git', 'init', '--bare', '-q', str(repository)], check=True)
            result = subprocess.run(['git', '-c', 'core.excludesFile=/dev/null',
                '--git-dir=' + str(repository), '--work-tree=' + str(root),
                'ls-files', '--others', '--exclude-standard', '-z'], capture_output=True, check=True)
    files = sorted(set(name for name in result.stdout.decode().split('\0') if name))
    for name in files:
        relative = PurePosixPath(name)
        if (relative.is_absolute() or '..' in relative.parts or relative.parts[0] not in TOP_LEVEL
                or PRIVATE_PARTS.intersection(relative.parts)
                or relative.suffix.lower() in PRIVATE_SUFFIXES
                or (relative.name.startswith('.env') and relative.name != '.env.example')
                or relative.name == 'jobs.json'):
            raise ValueError(f'Remove private/generated file from the publication index: {name}')
        path = root / relative
        # Check every component, not only the final file.
        current = root
        for part in relative.parts:
            current = current / part
            if current.is_symlink():
                raise ValueError(f'Symlinks cannot be published: {name}')
        if not path.is_file():
            raise ValueError(f'Publication input is not a regular file: {name}')
        with path.open('rb') as stream:
            magic = stream.read(4)
        if magic in BINARY_MAGIC or magic[:2] == b'MZ':
            raise ValueError(f'Compiled executable cannot be published as source: {name}')
    return files

def build_archive(root, destination):
    root = Path(root).resolve()
    files = candidate_files(root)
    with zipfile.ZipFile(destination, 'w', zipfile.ZIP_DEFLATED) as archive:
        for name in files:
            info = zipfile.ZipInfo('telegram-media-downloader/' + name)
            mode = 0o755 if Path(root / name).stat().st_mode & stat.S_IXUSR else 0o644
            info.external_attr = (stat.S_IFREG | mode) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, (root / name).read_bytes())
    return files

def stage(root, destination):
    destination = Path(destination)
    if destination.exists():
        raise ValueError('Use a new, empty publication staging path')
    files = candidate_files(root)
    destination.mkdir(parents=True)
    for name in files:
        path = destination / name
        path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(Path(root) / name, path)
    return files

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--zip', type=Path)
    group.add_argument('--stage', type=Path)
    options = parser.parse_args()
    files = build_archive(options.root, options.zip) if options.zip else stage(options.root, options.stage)
    print(f'Validated {len(files)} public source files.')
