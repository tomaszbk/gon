#!/usr/bin/env python3
"""Executable compatibility pairs plus native SSA execution and Staticcheck IR."""
import os
from pathlib import Path
import subprocess
ROOT = Path(__file__).resolve().parents[2]
GON = ROOT / 'gon/bin/gon'
ENV = dict(os.environ, GON_ROOT=str(ROOT))
baseline = ENV.get('GON_BASELINE_GO')
if not baseline:
    raise SystemExit('Set GON_BASELINE_GO to an unmodified Go toolchain')
def run(*args, cwd=ROOT):
    subprocess.run([str(a) for a in args], cwd=cwd, env=ENV, check=True)
for tool, name in [(baseline, 'legacy'), (GON, 'legacy'), (GON, 'modern')]:
    run(tool, 'run', ROOT / 'misc/gon/analysisfixtures' / (name + '.go'))
run(GON, 'test', './go/ssa', '-run=TestGonErrorFlow', '-count=1', cwd=ROOT / 'tools/x-tools')
run(GON, 'test', '-mod=mod', './go/ir', '-run=TestGonErrorFlow', '-count=1', cwd=ROOT / 'tools/staticcheck')
print('PASS: baseline, Gon legacy/modern, executed SSA legacy/modern, Staticcheck IR')
