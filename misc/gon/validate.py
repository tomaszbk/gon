#!/usr/bin/env python3
"""Run focused Gon integration gates. A passing subset is not feature completion."""
import argparse
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
GON = ROOT / 'gon/bin' / ('gon.exe' if os.name == 'nt' else 'gon')
MODERN_FEATURES = ('namedarguments', 'enums', 'matching', 'option', 'result')


def checks(feature):
    if feature == 'modern':
        steps = [(name, step) for name in MODERN_FEATURES for step in checks(name)]
        commands_by_name = {}
        for _, (name, cwd, cmd) in steps:
            commands_by_name.setdefault(name, set()).add((cwd, tuple(cmd)))
        selected, seen = [], set()
        for profile, (name, cwd, cmd) in steps:
            key = cwd, tuple(cmd)
            if key in seen:
                continue
            seen.add(key)
            if len(commands_by_name[name]) > 1:
                name = profile+'-'+name
            selected.append((name, cwd, cmd))
        return selected
    def test(name, cwd, packages, pattern=None):
        args = [str(GON), 'test', '-json', *packages]
        if pattern:
            args += ['-run', pattern]
        return name, cwd, args + ['-count=1']
    common = [
        ('vendor', '.', [sys.executable, 'misc/gon/vendor.py', '--check']),
        ('install-tools', '.', [str(GON), 'install', 'cmd/vet', 'cmd/fix', 'cmd/gofmt', 'cmd/cover', 'cmd/cgo', 'cmd/export']),
        ('build-tooling', '.', [sys.executable, 'misc/gon/build.py']),
        ('installer', '.', [sys.executable, 'misc/gon/test_install.py']),
        ('stringenums-vet', '.', [str(GON), 'vet', 'test/stringenums.dir/common.go', 'test/stringenums.dir/modern.go']),
        test('ast', '.', ['go/ast']),
        test('compiler-inline', '.', ['cmd/compile/internal/inline'], '^TestGon'),
        test('compiler-ssa', '.', ['cmd/compile/internal/ssacompile']),
        ('readme-examples', '.', [sys.executable, 'misc/gon/test_readme.py']),
        ('benchmark-correctness', '.', [sys.executable, 'misc/gon/benchmark.py', '--correctness-only']),
        test('structural-tools', 'tools/x-tools', ['./go/ast/inspector', './go/ast/astutil', './go/ast/edge', './go/cfg', './refactor/satisfy', './internal/typesinternal', './go/types/objectpath'], 'TestGon|TestCond|TestError|TestInspectAllNodes|TestEnumPaths|TestStringEnum'),
        test('analyzers', 'tools/gonpls', ['./internal/settings'], '^TestGonAnalyzers$'),
        test('refactor-safety', 'tools/x-tools', ['./internal/refactor/inline'], '^(TestGon|TestCalleeEffects|TestBasics|TestPrecedenceParens)'),
        test('staticcheck-safety', 'tools/staticcheck', ['./analysis/code', './go/ast/astutil', './go/types/typeutil'], '^TestGon'),
        test('optional-inference', 'tools/gonpls', ['./internal/golang', './internal/golang/completion'], '^TestGonOptional'),
    ]
    pairs = []
    if feature in ('enums', 'tooling'):
        common.append(test('sql', '.', ['database/sql', 'database/sql/driver']))
        if os.environ.get('GON_SQL_POSTGRES') == '1':
            common.append(('stringenums-postgres', '.', [sys.executable, 'misc/gon/test_stringenums_postgres.py']))
    features = ['errorhandling', 'errorbridge', 'errortest', 'conditional', 'lambda', 'nullsafety', 'namedarguments', 'enums', 'stringenums', 'stringenums_sql', 'matching', 'optionresult', 'optionsyntax'] if feature == 'tooling' else (['optionresult', 'optionsyntax'] if feature in ('option', 'result') else (['enums', 'stringenums', 'stringenums_sql'] if feature == 'enums' else (['errorhandling', 'errorbridge', 'errortest'] if feature == 'errorhandling' else [feature])))
    for name in features:
        pairs.append(test(name+'-execution', '.', ['cmd/internal/testdir'], 'Test/'+name+r'.go$'))
    if feature == 'tooling':
        return common + pairs + [
            test('ssa', 'tools/x-tools', ['./go/ssa'], '^TestGon'),
            test('staticcheck-ir', 'tools/staticcheck', ['./go/ir'], '^TestGon'),
            test('tooling-api', 'tools/gonpls', ['./internal/cmd'], '^TestGon'),
            test('typerefs', 'tools/gonpls', ['./internal/cache/typerefs'], '^TestRefs$'),
            test('unusedfunc', 'tools/gonpls', ['./internal/analysis/unusedfunc']),
            ('lsp', '.', [sys.executable, 'misc/gon/test.py']),
            ('namedarguments-lsp', '.', [sys.executable, 'misc/gon/test_namedarguments.py']),
            ('alternatives-lsp', '.', [sys.executable, 'misc/gon/test_alternatives.py']),
            ('stringenums-lsp', '.', [sys.executable, 'misc/gon/test_stringenums.py']),
            ('simplification-lsp', '.', [sys.executable, 'misc/gon/test_simplification.py']),
            test('export-data', 'tools/x-tools', ['./internal/gcimporter'], '^TestGon(Alternatives|StringEnums)'),
            ('cli', '.', [sys.executable, 'misc/gon/test_cli.py']),
            test('syntax-fixes', 'tools/x-tools', ['./go/analysis/passes/gonmodernize']),
            ('fix-execution', '.', [sys.executable, 'misc/gon/test_fix.py']),
        ]
    pattern = {'conditional': 'CondExpr|CondParen', 'errorhandling': 'ErrorHandling|ErrorExpr',
               'lambda': 'Lambda|NilSafety|NullSafety', 'nullsafety': 'Lambda|NilSafety|NullSafety',
               'namedarguments': 'NamedArguments', 'enums': 'Enum|Alternatives',
               'matching': 'Match|Alternatives', 'option': 'NativeOptional|OptionResult|OptionContext|Alternatives',
               'result': 'OptionResult|OptionContext|Alternatives'}[feature]
    extra = []
    if feature == 'conditional':
        extra = [
            test('cgo', '.', ['cmd/cgo/internal/testconditional'], '^Test(PairedCgoConditional|CgoConditionalDiagnostics|CgoConditionalBootstrap)$'),
            test('cover', '.', ['cmd/cover'], '^Test(CondFlowCoverage|ErrorFlowCoverage|ErrorHandlingRanges|LegacyInstrumentationUnchanged)$'),
            test('editor-query', 'tools/gonpls', ['./internal/cmd'], '^TestGonConditionalQuery$'),
            test('editor-extraction', 'tools/gonpls', ['./internal/golang'], '^TestConditionalExtraction$'),
            ('editor-lsp', '.', [sys.executable, 'misc/gon/test.py', '--conditional-only']),
        ]
    elif feature == 'errorhandling':
        extra = [
            test('cgo', '.', ['cmd/cgo/internal/testerrorhandling'], '^Test(PairedCgoErrorHandling|CgoErrorHandlingDiagnostics)$'),
            test('cover', '.', ['cmd/cover'], '^Test(ErrorFlowCoverage|ErrorHandlingRanges|LegacyInstrumentationUnchanged)$'),
            ('lsp', '.', [sys.executable, 'misc/gon/test.py']),
            ('cli', '.', [sys.executable, 'misc/gon/test_cli.py']),
        ]
    elif feature in ('lambda', 'nullsafety'):
        extra = [
            test('lexical', '.', ['go/token', 'go/scanner'], '^Test(GonTokens|Scan|Semis|ScanErrors)$'),
            test('cgo', '.', ['cmd/cgo', 'cmd/cgo/internal/testconditional'], '^Test(Gon|PairedCgoGonFeatures|CgoConditionalBootstrap)'),
            test('cover', '.', ['cmd/cover'], '^Test(GonFlowCoverage|GonFunctionBoundary|LegacyInstrumentationUnchanged)$'),
            test('editor-query', 'tools/gonpls', ['./internal/cmd'], '^TestGon(FeatureQuery|FeatureExplain)$'),
            test('editor-extraction', 'tools/gonpls', ['./internal/golang'], '^Test(GonFeatureExtraction|ConditionalExtraction)$'),
            test('typerefs', 'tools/gonpls', ['./internal/cache/typerefs'], '^TestRefs$'),
            ('editor-lsp', '.', [sys.executable, 'misc/gon/test.py', '--features-only']),
            test('analyzer-diagnostics', 'tools/x-tools', ['./go/analysis/passes/'+p for p in
                 ['copylock', 'lostcancel', 'nilfunc', 'defers', 'waitgroup', 'unusedresult', 'printf', 'testinggoroutine', 'unreachable']], '^TestGon'),
            test('staticcheck-diagnostics', 'tools/staticcheck', ['./internal/sharedcheck', './simple/s1023'] +
                 ['./staticcheck/'+p for p in ['sa4004', 'sa4009', 'sa5003', 'sa9001']], '^TestGon'),
        ]
    elif feature in ('namedarguments', 'enums', 'matching', 'option', 'result'):
        cgo_pattern = {'namedarguments': 'NamedArguments', 'enums': '(Matching|StringEnums)',
                       'matching': 'Matching', 'option': 'OptionResult', 'result': 'OptionResult'}[feature]
        fixture = 'optionresult' if feature in ('option', 'result') else feature
        extra = [
            test('lexical', '.', ['go/token', 'go/scanner'], '^Test(GonTokens|Scan|Semis|ScanErrors)$'),
            # Verify the adapted cgo source still builds against the baseline AST.
            test('cgo-bootstrap', '.', ['cmd/cgo/internal/testconditional'], '^TestCgoConditionalBootstrap$'),
            # Paired Cgo checks also compile and execute instrumented programs.
            test('cgo-cover', '.', ['cmd/cgo/internal/testconditional'], ('^Test(PairedCgo(Matching|StringEnums)|CgoStringEnumDiagnostics)$' if feature == 'enums' else '^TestPairedCgo'+cgo_pattern+'$')),
            ('vet', '.', [str(GON), 'vet', 'test/'+fixture+'.dir/common.go', 'test/'+fixture+'.dir/modern.go']),
            test('explain', 'tools/gonpls', ['./internal/cmd'], '^TestGon(Explain|FeatureExplain|AlternativesQuery)$'),
            ('editor-lsp', '.', [sys.executable, 'misc/gon/test_namedarguments.py' if feature == 'namedarguments' else 'misc/gon/test_alternatives.py']),
        ]
        if feature != 'namedarguments':
            extra.append(test('export-data', 'tools/x-tools', ['./internal/gcimporter'], '^TestGon(Alternatives|StringEnums)'))
        if feature == 'option':
            extra.append(test('optional-migration', 'tools/gonpls', ['./internal/cmd'], '^TestGon(NativeOptionalMigration|OptionalMigrationPlan)$'))
        if feature in ('option', 'result'):
            extra.append(('simplification-lsp', '.', [sys.executable, 'misc/gon/test_simplification.py']))
        if feature == 'enums':
            extra.append(('stringenums-lsp', '.', [sys.executable, 'misc/gon/test_stringenums.py']))
            extra.append(('representation', '.', [str(GON), 'test', '-json', 'test/enums.dir/representation_test.go',
                         '-run', '^TestAlternativeRepresentation$', '-bench', '^BenchmarkAlternatives$',
                         '-benchtime=100ms', '-benchmem', '-count=1']))
    return common + pairs + extra + [
        test('syntax', '.', ['cmd/compile/internal/syntax', 'go/parser', 'go/printer', 'go/format', 'cmd/gofmt'], pattern),
        test('types', '.', ['cmd/compile/internal/types2', 'go/types'], pattern+'|TestGenerate'),
        test('ssa', 'tools/x-tools', ['./go/ssa'], '^TestGon'),
        test('staticcheck-ir', 'tools/staticcheck', ['./go/ir'], '^TestGon'),
        # New feature pairs exercise vet directly in their execution harnesses.
        *([test('vet', '.', ['cmd/vet'], '^TestCondExpr$' if feature == 'conditional' else '^TestVet$')]
          if feature in ('conditional', 'errorhandling') else []),
    ]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('feature', choices=['tooling', 'errorhandling', 'conditional', 'lambda', 'nullsafety', *MODERN_FEATURES, 'modern'])
    parser.add_argument('--list', action='store_true', help='show commands without executing')
    parser.add_argument('--only', action='append', help='run selected check IDs; reports a partial run')
    args = parser.parse_args()
    plan = checks(args.feature)
    if args.only:
        unknown = set(args.only) - {name for name, _, _ in plan}
        if unknown:
            parser.error('unknown checks: ' + ', '.join(sorted(unknown)))
        plan = [step for step in plan if step[0] in args.only]
    if args.list:
        for name, cwd, cmd in plan:
            print(f'{name}: (cd {cwd} && {shlex.join(cmd)})')
        return 0
    baseline = os.environ.get('GON_BASELINE_GO') or os.environ.get('GO_ERROR_HANDLING_BASELINE')
    if not baseline or not Path(baseline).is_absolute() or not os.access(baseline, os.X_OK):
        parser.error('set GON_BASELINE_GO to an unmodified compatible Go executable (absolute path)')
    env = dict({k: v for k, v in os.environ.items() if k not in ('GOROOT', 'GOTOOLDIR')}, GON_ROOT=str(ROOT), GOWORK='off',
               GOTOOLCHAIN='local', GOFLAGS='', GON_BASELINE_GO=baseline,
               GO_ERROR_HANDLING_BASELINE=baseline, GO_CONDITIONAL_EXPRESSION_BASELINE=baseline)
    # Analysis loaders invoke "go"; select the private tool only in child processes.
    env['PATH'] = str(ROOT / 'bin') + os.pathsep + env.get('PATH', '')
    out = ROOT / 'pkg/gon-validation' / args.feature
    out.mkdir(parents=True, exist_ok=True)
    results = []
    for index, (name, cwd, cmd) in enumerate(plan):
        print(f'RUN {name}: {shlex.join(cmd)}', flush=True)
        start = time.monotonic()
        log = out / (name+'.log')
        with log.open('w') as stream:
            proc = subprocess.run(cmd, cwd=ROOT/cwd, env=env, stdout=stream, stderr=subprocess.STDOUT)
        result = dict(check=name, command=cmd, cwd=cwd, status='pass' if proc.returncode == 0 else 'fail',
                      exitCode=proc.returncode, seconds=round(time.monotonic()-start, 3), log=str(log))
        if len(cmd) > 1 and cmd[1] == 'test':
            events = []
            for line in log.read_text().splitlines():
                try:
                    events.append(json.loads(line))
                except ValueError:
                    pass
            result['testsRun'] = sum(e.get('Action') == 'run' for e in events)
            result['skippedTests'] = [e.get('Package', '') + '/' + e['Test'] for e in events
                                      if e.get('Action') == 'skip' and e.get('Test')]
            if not result['testsRun']:
                result['status'] = 'fail'
                result['reason'] = 'no matching tests ran'
        results.append(result)
        print(f'{result["status"].upper()} {name} ({result["seconds"]}s): {log}', flush=True)
        if result['status'] == 'fail':
            print(log.read_text()[-10000:], flush=True)
            if name in ('vendor', 'install-tools', 'build-tooling'):
                results.extend(dict(check=n, command=c, cwd=d, status='not-run', reason='setup failed')
                               for n, d, c in plan[index+1:])
                break
    inventory = json.loads((ROOT/'misc/gon/features.json').read_text())
    pending = ([name+': '+item for name in MODERN_FEATURES for item in inventory[name]['pending']]
               if args.feature == 'modern' else inventory[args.feature]['pending'])
    summary = dict(feature=args.feature, partial=bool(args.only), pending=pending, results=results)
    (out/'summary.json').write_text(json.dumps(summary, indent=2)+'\n')
    if pending:
        print('FEATURE STILL OPEN: ' + '; '.join(pending))
    if args.only:
        print('PARTIAL RUN: unselected checks were not executed')
    if any(r['status']=='fail' for r in results):
        return 1
    print('PASS: selected checks' if args.only or pending else 'PASS: all integration gates')
    return 2 if pending or args.only else 0


if __name__ == '__main__':
    sys.exit(main())
