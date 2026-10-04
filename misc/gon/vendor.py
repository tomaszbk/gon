#!/usr/bin/env python3
"""Generate std and cmd vendor trees from maintained sources; --check detects drift.

Edit tools/x-tools, never its generated std or cmd vendor copies. Other
modules retain the versions selected by each go.mod. No Gon patches are applied.
"""
import argparse
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
GO = ROOT / 'gon/bin' / ('gon.exe' if os.name == 'nt' else 'gon')
ENV = dict(os.environ, GOROOT=str(ROOT), GOTOOLCHAIN='local', GOWORK='off', GOFLAGS='')


def files(root):
    return {p.relative_to(root): p.read_bytes() for p in root.rglob('*') if p.is_file()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    drift = []
    for module, label in [('src', 'std'), ('src/cmd', 'cmd')]:
        module_root = ROOT / module
        target = module_root / 'vendor'
        with tempfile.TemporaryDirectory(prefix='gon-vendor-') as tmp:
            generated = Path(tmp) / 'vendor'
            subprocess.run([str(GO), 'mod', 'vendor', '-o', str(generated)],
                           cwd=module_root, env=ENV, check=True)
            before, after = files(target), files(generated)
            changed = sorted(str(p) for p in before.keys() | after.keys()
                             if before.get(p) != after.get(p))
            if args.check:
                drift.extend(f'{module}/vendor/{p}' for p in changed)
            else:
                # Write only differences in the generated dependency trees.
                for rel in before.keys() - after.keys():
                    (target / rel).unlink()
                for rel, data in after.items():
                    if before.get(rel) != data:
                        path = target / rel
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_bytes(data)
                print(f'Generated {label} vendor ({len(changed)} changed files)')
    if args.check:
        if drift:
            raise SystemExit('Vendor differs; run python3 misc/gon/vendor.py:\n' + '\n'.join(drift))
        print('PASS: vendor matches maintained sources')


if __name__ == '__main__':
    main()
