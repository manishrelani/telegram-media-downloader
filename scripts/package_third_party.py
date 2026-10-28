"""Bundle exact Go dependency source archives and module definitions for rebuilding."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]

def json_stream(raw):
    decoder = json.JSONDecoder()
    while raw.strip():
        raw = raw.lstrip()
        value, end = decoder.raw_decode(raw)
        yield value
        raw = raw[end:]

def package(destination):
    destination = Path(destination)
    destination.mkdir(parents=True, exist_ok=True)
    records = {}
    for module in ['thumbnail-helper', 'runtime/tdl']:
        module_root = ROOT / module
        definitions = destination / module
        definitions.mkdir(parents=True, exist_ok=True)
        downloads = subprocess.run(['go', 'mod', 'download', '-json', 'all'],
                                   cwd=module_root, text=True, capture_output=True, check=True)
        for file in module_root.iterdir():
            if file.name in {'go.mod', 'go.sum', 'LICENSE'} or file.suffix == '.go':
                shutil.copyfile(file, definitions / file.name)
        for info in json_stream(downloads.stdout):
            if info.get('Error'):
                raise RuntimeError(info['Error'])
            name = info['Path'] + '@' + info['Version']
            if name in records:
                continue
            archive = Path(info['Zip'])
            # Full module path, not just its final component, prevents collisions.
            target = destination / 'modules' / (info['Path'] + '@' + info['Version'] + '.zip')
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(archive, target)
            records[name] = {'path': target.relative_to(destination).as_posix(),
                             'go_sum': info['Sum'],
                             'sha256': hashlib.sha256(target.read_bytes()).hexdigest()}
    (destination / 'sources.json').write_text(json.dumps(records, indent=2, sort_keys=True) + '\n')
    (destination / 'REBUILD.txt').write_text(
        'Exact dependency sources for the bundled tdl runtime and preview helper.\n'
        'Each module ZIP includes its upstream license files. The tdl executable and\n'
        'preview helper use AGPL-3.0; see their source archives and bundled notices.\n'
        'Use Go 1.27.3 and the included go.mod/go.sum files.\n'
        'runtime/tdl: go build -trimpath -ldflags="-w -X github.com/iyear/tdl/pkg/consts.Version=0.20.4 -X github.com/iyear/tdl/pkg/consts.Commit=security-gotd-0.145.1" github.com/iyear/tdl\n'
        'thumbnail-helper: go build -trimpath -ldflags="-w" .\n'
        'Source patches: dependency upgrades pinned in the module definitions.\n'
        'No account credentials or Telegram session data are included.\n')

if __name__ == '__main__':
    package(sys.argv[1])
