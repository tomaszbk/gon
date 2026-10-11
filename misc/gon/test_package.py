#!/usr/bin/env python3
"""Extract an actual release archive and check relocation and paired execution."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive', type=Path)
    parser.add_argument('--baseline', default=os.environ.get('GON_BASELINE_GO'))
    args = parser.parse_args()
    if not args.baseline or not Path(args.baseline).is_absolute():
        parser.error('supply an absolute unmodified Go baseline')
    environment = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off', GOFLAGS='')
    for name in ('GOROOT', 'GOTOOLDIR', 'GOOS', 'GOARCH'):
        environment.pop(name, None)
    with tempfile.TemporaryDirectory(prefix='gon release & spaces ') as temporary:
        folder = Path(temporary)
        if args.archive.suffix == '.zip':
            with zipfile.ZipFile(args.archive) as archive:
                names = archive.namelist()
                archive.extractall(folder)
        else:
            with tarfile.open(args.archive) as archive:
                names = archive.getnames()
                archive.extractall(folder, filter='data')
        for name in names:
            parts = Path(name).parts[1:]
            if '.git' in parts or 'design' in parts or parts == ('AGENTS.md',):
                raise SystemExit(f'Local-only file in archive: {name}')
        root = next(folder.iterdir())
        release = json.loads((root / 'GON_RELEASE.json').read_text())
        suffix = '.exe' if os.name == 'nt' else ''
        gon = root / 'gon/bin' / ('gon' + suffix)
        gonpls = root / 'gon/bin' / ('gonpls' + suffix)

        def run(tool, *args, cwd=root):
            return subprocess.check_output([str(tool), *args], cwd=cwd,
                                           env=environment, text=True)

        capabilities = json.loads(run(gon, 'capabilities', '--json'))
        if not capabilities:
            raise SystemExit('Empty capabilities')
        run(gonpls, 'version')
        actual_root = Path(run(gon, 'env', 'GOROOT').strip()).resolve()
        if actual_root != root.resolve():
            raise SystemExit(f'Archive used another toolchain: {actual_root}')
        baseline_root = Path(run(args.baseline, 'env', 'GOROOT').strip()).resolve()
        if baseline_root == actual_root or 'devel' in run(args.baseline, 'version'):
            raise SystemExit('Baseline must be a separate released Go')
        project = folder / 'application'
        project.mkdir()
        (project / 'go.mod').write_text('module example.com/gon-release\n\ngo 1.27\n')
        fixtures = ROOT / 'misc/gon/readme-examples'
        shutil.copyfile(fixtures / 'common.go', project / 'common.go')
        shutil.copyfile(fixtures / 'legacy.go', project / 'source.go')
        expected = run(args.baseline, 'run', '.', cwd=project)
        if run(gon, 'run', '.', cwd=project) != expected:
            raise SystemExit('Archive legacy behavior differs from baseline')
        shutil.copyfile(fixtures / 'modern.go', project / 'source.go')
        if run(gon, 'run', '.', cwd=project) != expected:
            raise SystemExit('Archive modern behavior differs from baseline')
        run(gon, 'check', './...', cwd=project)
        run(gon, 'vet', './...', cwd=project)
        run(gon, 'fmt', './...', cwd=project)
        print('PASS: relocated archive, gon/gonpls, semantic checks, fmt/vet and executable Go/Gon pairs; '
              + release['platform'] + ' source ' + release['sourceCommit'])


if __name__ == '__main__':
    main()
