"""Fail-closed publication, secret and dependency checks. Requires pinned tools in PATH."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]

def check_osv(payload, expected):
    results = payload.get('results')
    if not isinstance(results, list) or len(results) != expected:
        raise RuntimeError('OSV returned an incomplete response')
    for result in results:
        if not isinstance(result, dict) or result.get('error') or result.get('vulns'):
            raise RuntimeError('OSV reported a vulnerability or a query failure; inspect the private report')

def pub_dependencies(lockfile):
    packages = []
    for block in re.split(r'(?m)^  (?=\S)', lockfile.read_text())[1:]:
        if re.search(r'(?m)^    source: hosted$', block):
            name = block.split(':', 1)[0]
            match = re.search(r'(?m)^    version: "([^"]+)"$', block)
            if not match:
                raise RuntimeError('Unrecognized hosted dependency in lockfile')
            packages.append({'package': {'name': name, 'ecosystem': 'Pub'}, 'version': match[1]})
    if not packages:
        raise RuntimeError('No locked Pub dependencies found')
    return packages

def run(command, cwd=ROOT, env=None):
    subprocess.run(command, cwd=cwd, env=env, check=True)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report-dir', type=Path, default=ROOT/'build/security-checks')
    args = parser.parse_args()
    report = args.report_dir.resolve()
    report.mkdir(parents=True, exist_ok=True)
    spec = importlib.util.spec_from_file_location('public_source', ROOT/'scripts/public_source.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    with tempfile.TemporaryDirectory(prefix='tmm-public-security-') as temporary:
        staged = Path(temporary)/'source'
        files = module.stage(ROOT, staged)
        (report/'public-files.json').write_text(json.dumps(files, indent=2)+'\n')
        run(['gitleaks', 'dir', str(staged), '--redact', '--no-banner',
             '--report-format', 'json', '--report-path', str(report/'secrets.json')])
    if (ROOT/'.git').exists():
        run(['gitleaks', 'git', str(ROOT), '--redact', '--no-banner', '--log-opts=--all',
             '--report-format', 'json', '--report-path', str(report/'history-secrets.json')])
    environment = dict(os.environ, GOTOOLCHAIN='go1.27.3')
    for module_path, target in [('backend','./...'), ('thumbnail-helper','./...'), ('runtime/tdl','github.com/iyear/tdl')]:
        # Package scanning rejects vulnerable imported packages even if current
        # call-graph analysis does not find a reachable vulnerable symbol.
        run(['govulncheck', '-scan', 'package', target], cwd=ROOT/module_path, env=environment)
    packages = pub_dependencies(ROOT/'apps/desktop/pubspec.lock')
    request = urllib.request.Request('https://api.osv.dev/v1/querybatch',
              data=json.dumps({'queries': packages}).encode(), headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(request, timeout=60) as response:
        payload = json.load(response)
    (report/'pub-osv.json').write_text(json.dumps(payload, indent=2)+'\n')
    check_osv(payload, len(packages))
    print(f'Security checks passed: {len(files)} publication files; {len(packages)} Pub packages; 3 Go targets.')

if __name__ == '__main__':
    main()
