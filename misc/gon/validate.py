#!/usr/bin/env python3
"""Run focused Gon integration gates. A passing subset is not feature completion."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shlex
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
GON = ROOT / 'gon/bin' / ('gon.exe' if os.name == 'nt' else 'gon')
MODERN_FEATURES = ('namedarguments', 'enums', 'matching', 'option', 'seq', 'errorcontext', 'matchalternatives', 'patterntest', 'interpolation', 'nilanalysis')
_source_hashes = {}


def tool_snapshot(env):
    target = subprocess.check_output([str(GON), 'env', 'GOOS', 'GOARCH'], env=env, text=True).split()
    private = ROOT / 'pkg/tool' / '_'.join(target)
    paths = [ROOT/'bin/go', ROOT/'bin/gofmt', GON, ROOT/'gon/bin/gonpls',
             *[private/name for name in ('compile', 'link', 'vet', 'fix', 'cover', 'cgo', 'gonpls')]]
    return {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in paths if path.is_file()}


def source_snapshot():
    """Fingerprint implementation, fixtures and API inventories, including edits.

    Documentation and retained run evidence are excluded so recording a result
    does not invalidate it. Ignored build outputs and local design notes are not
    inputs. The file manifest makes an old run's exact scope inspectable.
    """
    paths = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=ROOT).decode().split('\0')
    files = {}
    for name in sorted(set(paths) - {''}):
        if name.endswith('.md') or name.startswith('misc/gon/benchmarks/results/') or name.startswith('misc/gon/validation/'):
            continue
        path = ROOT / name
        try:
            st = path.lstat()
            stamp = (st.st_size, st.st_mtime_ns, st.st_ctime_ns, st.st_ino)
        except FileNotFoundError:
            stamp = None
        cached = _source_hashes.get(name)
        if cached is not None and cached[0] == stamp:
            files[name] = cached[1]
            continue
        if path.is_symlink():
            data = ('symlink:'+os.readlink(path)).encode()
        elif path.is_file():
            data = path.read_bytes()
        else:
            data = b'<deleted>'
        files[name] = hashlib.sha256(data).hexdigest()
        _source_hashes[name] = (stamp, files[name])
    digest = hashlib.sha256(json.dumps(files, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    return dict(sha256=digest, files=files)


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
        ('validation-snapshot', '.', [sys.executable, 'misc/gon/test_validate.py']),
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
        test('tooling-docs', 'tools/gonpls', ['./internal/doc/generate'], '^TestGenerated$'),
        test('refactor-safety', 'tools/x-tools', ['./internal/refactor/inline'], '^(TestGon|TestCalleeEffects|TestBasics|TestPrecedenceParens)'),
        test('staticcheck-safety', 'tools/staticcheck', ['./analysis/code', './go/ast/astutil', './go/types/typeutil'], '^TestGon'),
        test('staticcheck-unused', 'tools/staticcheck', ['./unused'], '^TestGonUnused$'),
        ('public-api-host', '.', [str(GON), 'test', '-json', 'cmd/api', '-run', '^(TestCheck|TestGonParameterAPI|TestGonNestedParameterNames|TestGonReachableParameterNames)$', '-count=1', '-args', '-check', '-host']),
        test('optional-link-identity', '.', ['cmd/compile/internal/types'], '^TestGonOptionalShapeLinkIdentity$'),
        test('optional-dwarf', '.', ['cmd/link/internal/ld'], '^TestGonOptionalShapeDWARF$'),
        test('cgo-errno-check', 'tools/gonpls', ['./internal/cmd'], '^TestGonCgoErrorHandlingCheck$'),
        test('gon-namespace-build', '.', ['go/build'], '^TestGonPackagePrecedence$'),
        test('gon-namespace-execution', '.', ['cmd/go'], '^TestGonNamespacePair$|^TestScript/gon_namespace$'),
        test('optional-inference', 'tools/gonpls', ['./internal/golang', './internal/golang/completion'], '^TestGonOptional'),
        test('optional-hover', 'tools/gonpls', ['./internal/test/integration/misc'], '^TestGonOptionalHover$'),
        test('enum-layout-types', '.', ['cmd/compile/internal/types2', 'go/types'], '^Test(EnumUnitLayout|Generate)$'),
        test('error-return-types', '.', ['cmd/compile/internal/types2', 'go/types'], 'ErrorHandling|ErrorExpr|ErrorBridge'),
    ]
    if feature in ('matchalternatives', 'patterntest'):
        name = 'MatchAlternatives' if feature == 'matchalternatives' else 'PatternTest'
        return [
            test(feature+'-syntax', '.', ['cmd/compile/internal/syntax', 'go/parser', 'go/ast', 'go/printer', 'go/format'], name+'|TestChildren'),
            test(feature+'-types', '.', ['cmd/compile/internal/types2', 'go/types'], name+'|TestGenerate'),
            test(feature+'-execution', '.', ['cmd/internal/testdir'], 'Test/'+feature+r'\.go$'),
            test(feature+'-cgo', '.', ['cmd/cgo/internal/testconditional'], '^TestPairedCgoMatching$'),
            test(feature+'-ssa', 'tools/x-tools', ['./go/ssa'], '^TestGon'+name),
            test(feature+'-ir', 'tools/staticcheck', ['./go/ir'], '^TestGon'+name),
            test('analyzers', 'tools/gonpls', ['./internal/settings'], '^TestGonAnalyzers$'),
        ]
    if feature == 'interpolation':
        return [
            test('interpolation-syntax', '.', ['cmd/compile/internal/syntax', 'go/scanner', 'go/token', 'go/parser', 'go/ast', 'go/printer', 'go/format'], 'Interpolation|TestChildren'),
            test('interpolation-types', '.', ['cmd/compile/internal/types2', 'go/types'], 'Interpolation|TestExprString'),
            test('interpolation-execution', '.', ['cmd/internal/testdir'], r'Test/interpolation\.go$'),
            test('interpolation-cgo-cover', '.', ['cmd/cgo/internal/testconditional'], '^TestPairedCgoInterpolation$'),
            test('interpolation-ssa', 'tools/x-tools', ['./go/ssa'], '^TestGonInterpolation'),
            test('interpolation-ir', 'tools/staticcheck', ['./go/ir'], '^TestGonInterpolation'),
            test('interpolation-printf', 'tools/x-tools', ['./go/analysis/passes/printf'], '^Test'),
            test('interpolation-copylocks', 'tools/x-tools', ['./go/analysis/passes/copylock'], '^Test'),
            test('interpolation-imports', 'tools/x-tools', ['./internal/imports'], '^TestGonInterpolationImports$'),
            test('analyzers', 'tools/gonpls', ['./internal/settings'], '^TestGonAnalyzers$'),
            test('interpolation-editor', 'tools/gonpls', ['./internal/test/marker'], '^Test/quickfix/interpolation'),
        ]
    if feature == 'errorcontext':
        return [
            test('errorcontext-syntax', '.', ['cmd/compile/internal/syntax', 'go/parser', 'go/ast', 'go/printer', 'go/format'], 'ErrorContext|TestChildren'),
            test('errorcontext-types', '.', ['cmd/compile/internal/types2', 'go/types'], 'ErrorContext'),
            test('errorcontext-execution', '.', ['cmd/internal/testdir'], r'Test/errorcontext\.go$'),
            test('errorcontext-cgo', '.', ['cmd/cgo/internal/testconditional'], '^TestPairedCgoErrorContext$'),
            test('errorcontext-cover', '.', ['cmd/cover'], '^TestErrorContextCoverage$'),
            test('errorcontext-ssa', 'tools/x-tools', ['./go/ssa'], '^TestGonErrorContext'),
            test('errorcontext-ir', 'tools/staticcheck', ['./go/ir'], '^TestGonErrorContext'),
            test('errorcontext-modernize', 'tools/x-tools', ['./go/analysis/passes/gonmodernize'], '^Test'),
            test('errorcontext-query', 'tools/gonpls', ['./internal/cmd'], '^TestGon(AlternativesQuery|TestPropagationQuery)$'),
        ]
    if feature == 'nilanalysis':
        return [
            ('nilaway-corpus', 'tools/nilaway', [str(GON), 'test', '-json', '-p=1', '-parallel=2', './...', '-count=1']),
            test('nilaway-gon', 'tools/nilaway', ['.'], '^TestGon'),
            test('nilaway-settings', 'tools/gonpls', ['./internal/settings'], '^(TestNilAwayOptIn|TestGonAnalyzers)$'),
            test('nilaway-cli', 'tools/gonpls', ['./internal/cmd'], '^TestGonNilAway'),
            test('nilanalysis-execution', '.', ['cmd/internal/testdir'], r'Test/nilanalysis\.go$'),
        ]
    if feature == 'seq':
        return [
            test('seq-unit', '.', ['gon/seq']),
            ('seq-dwarf-binary', '.', [str(GON), 'test', '-c', 'gon/seq', '-o', str(ROOT/'pkg/gon-validation/seq/seq.test')]),
            ('seq-dwarf-execution', '.', [str(ROOT/'pkg/gon-validation/seq/seq.test')]),
            ('seq-api-host', '.', [str(GON), 'test', '-json', 'cmd/api', '-run', '^(TestCheck|TestGonParameterAPI)$', '-count=1', '-args', '-check', '-host']),
            test('seq-deps', '.', ['go/build'], '^TestDependencies$'),
            test('seq-execution', '.', ['cmd/internal/testdir'], r'Test/seq\.go$'),
            ('seq-vet', '.', [str(GON), 'vet', 'test/seq.dir/common.go', 'test/seq.dir/modern.go']),
        ]
    pairs = []
    if feature in ('enums', 'tooling'):
        common.append(test('sql', '.', ['database/sql', 'database/sql/driver']))
        if os.environ.get('GON_SQL_POSTGRES') == '1':
            common.append(('stringenums-postgres', '.', [sys.executable, 'misc/gon/test_stringenums_postgres.py']))
    features = ['errorhandling', 'errortest', 'conditional', 'lambda', 'nullsafety', 'namedarguments', 'enums', 'stringenums', 'stringenums_sql', 'matching', 'matchinterface', 'optionals', 'optionsyntax', 'seq', 'errorcontext', 'matchalternatives', 'patterntest', 'interpolation', 'nilanalysis'] if feature == 'tooling' else (['optionals', 'optionsyntax'] if feature == 'option' else (['enums', 'stringenums', 'stringenums_sql'] if feature == 'enums' else (['matching', 'matchinterface'] if feature == 'matching' else (['errorhandling', 'errortest'] if feature == 'errorhandling' else [feature]))))
    if feature in ('option', 'tooling'):
        # Native optionals at the JSON, SQL and reflection boundaries. The v1
        # JSON implementation is checked separately: the v2 based one is the default.
        features += ['optionaljson', 'optionalsql', 'sqljson']
        common += [
            test('optional-json', '.', ['encoding/json', 'encoding/json/v2'], 'GonOptional'),
            ('optional-json-v1', '.', ['env', 'GOEXPERIMENT=nojsonv2', str(GON), 'test', '-json', 'encoding/json', '-run', 'GonOptional', '-count=1']),
            test('optional-sql', '.', ['database/sql', 'database/sql/driver'], 'GonOptional|GonJSON'),
            test('optional-reflect', '.', ['reflect'], '^TestOptional'),
            ('optionaljson-vet', '.', [str(GON), 'vet', 'test/optionaljson.dir/common.go', 'test/optionaljson.dir/modern.go']),
            ('optionalsql-vet', '.', [str(GON), 'vet', 'test/optionalsql.dir/common.go', 'test/optionalsql.dir/modern.go']),
            ('sqljson-vet', '.', [str(GON), 'vet', 'test/sqljson.dir/common.go', 'test/sqljson.dir/modern.go']),
        ]
        if os.environ.get('GON_SQL_POSTGRES') == '1':
            common.append(('optionals-postgres', '.', [sys.executable, 'misc/gon/test_optionals_postgres.py']))
    for name in features:
        pairs.append(test(name+'-execution', '.', ['cmd/internal/testdir'], 'Test/'+name+r'.go$'))
    if feature == 'tooling':
        pairs.append(test('sqlstruct-execution', '.', ['cmd/internal/testdir'], r'Test/sqlstruct\.go$'))
    if feature == 'tooling':
        return common + pairs + [
            test('ssa', 'tools/x-tools', ['./go/ssa'], '^TestGon'),
            test('staticcheck-ir', 'tools/staticcheck', ['./go/ir'], '^TestGon'),
            test('tooling-api', 'tools/gonpls', ['./internal/cmd'], '^TestGon'),
            test('typerefs', 'tools/gonpls', ['./internal/cache/typerefs'], '^TestRefs$'),
            test('unusedfunc', 'tools/gonpls', ['./internal/analysis/unusedfunc']),
            test('vet-error-handlers', 'tools/x-tools', ['./go/analysis/passes/httpresponse', './go/analysis/passes/sqlrowserr'], '^TestGon$'),
            test('inline-variable-gon', 'tools/gonpls', ['./internal/test/marker'], '^Test/codeaction/inline-var-gon'),
            ('lsp', '.', [sys.executable, 'misc/gon/test.py']),
            ('namedarguments-lsp', '.', [sys.executable, 'misc/gon/test_namedarguments.py']),
            ('alternatives-lsp', '.', [sys.executable, 'misc/gon/test_alternatives.py']),
            ('interfacematch-lsp', '.', [sys.executable, 'misc/gon/test_interfacematch.py']),
            ('stringenums-lsp', '.', [sys.executable, 'misc/gon/test_stringenums.py']),
            ('simplification-lsp', '.', [sys.executable, 'misc/gon/test_simplification.py']),
            test('export-data', 'tools/x-tools', ['./internal/gcimporter'], '^TestGon(Alternatives|StringEnums)'),
            ('cli', '.', [sys.executable, 'misc/gon/test_cli.py']),
            test('syntax-fixes', 'tools/x-tools', ['./go/analysis/passes/gonmodernize']),
            ('fix-execution', '.', [sys.executable, 'misc/gon/test_fix.py']),
        ]
    pattern = {'conditional': 'CondExpr|CondParen', 'errorhandling': 'ErrorHandling|ErrorExpr|ErrorBridge',
               'lambda': 'Lambda|NilSafety|NullSafety', 'nullsafety': 'Lambda|NilSafety|NullSafety',
               'namedarguments': 'NamedArguments', 'enums': 'Enum|Alternatives',
               'matching': 'Match|Alternatives', 'option': 'NativeOptional|OptionalOperators|OptionContext|Alternatives'}[feature]
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
    elif feature in ('namedarguments', 'enums', 'matching', 'option'):
        cgo_pattern = {'namedarguments': 'NamedArguments', 'enums': '(Matching|StringEnums)',
                       'matching': 'Matching', 'option': 'Optional'}[feature]
        fixture = 'optionals' if feature == 'option' else feature
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
            extra.append(('simplification-lsp', '.', [sys.executable, 'misc/gon/test_simplification.py']))
        if feature == 'matching':
            extra.append(('matchinterface-vet', '.', [str(GON), 'vet', 'test/matchinterface.dir/common.go', 'test/matchinterface.dir/modern.go']))
            extra.append(('interfacematch-lsp', '.', [sys.executable, 'misc/gon/test_interfacematch.py']))
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
    baseline = os.environ.get('GON_BASELINE_GO')
    if not baseline or not Path(baseline).is_absolute() or not os.access(baseline, os.X_OK):
        parser.error('set GON_BASELINE_GO to an unmodified compatible Go executable (absolute path)')
    env = dict({k: v for k, v in os.environ.items() if k not in ('GOROOT', 'GOTOOLDIR')}, GON_ROOT=str(ROOT), GOWORK='off',
               GOTOOLCHAIN='local', GOFLAGS='', GON_BASELINE_GO=baseline)
    # Analysis loaders invoke "go"; select the private tool only in child processes.
    env['PATH'] = str(ROOT / 'bin') + os.pathsep + env.get('PATH', '')
    out = ROOT / 'pkg/gon-validation' / args.feature
    out.mkdir(parents=True, exist_ok=True)
    snapshot = source_snapshot()
    (out/'source-start.json').write_text(json.dumps(snapshot, indent=2)+'\n')
    head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    baseline_version = subprocess.check_output([baseline, 'version'], env=env, text=True).strip()
    results = []
    for index, (name, cwd, cmd) in enumerate(plan):
        print(f'RUN {name}: {shlex.join(cmd)}', flush=True)
        start = time.monotonic()
        log = out / (name+'.log')
        with log.open('w') as stream:
            proc = subprocess.run(cmd, cwd=ROOT/cwd, env=env, stdout=stream, stderr=subprocess.STDOUT)
        result = dict(check=name, command=cmd, cwd=cwd, status='pass' if proc.returncode == 0 else 'fail',
                      exitCode=proc.returncode, seconds=round(time.monotonic()-start, 3), log=str(log))
        result['tools'] = tool_snapshot(env)
        # A compile-only test command produces a binary, not test events.
        if len(cmd) > 1 and cmd[1] == 'test' and '-c' not in cmd:
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
        current = source_snapshot()
        result['sourceSHA256'] = current['sha256']
        if current['sha256'] != snapshot['sha256']:
            result['status'] = 'fail'
            result['reason'] = 'implementation changed during validation'
            results.extend(dict(check=n, command=c, cwd=d, status='not-run', reason='source changed')
                           for n, d, c in plan[index+1:])
            print('FAIL: implementation changed during validation; rerun on a stable snapshot', flush=True)
            break
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
    final_snapshot = source_snapshot()
    (out/'source-end.json').write_text(json.dumps(final_snapshot, indent=2)+'\n')
    summary = dict(feature=args.feature, partial=bool(args.only), pending=pending, results=results,
                   gitHEAD=head, platform=dict(sysplatform=sys.platform, machine=platform.machine()),
                   baseline=dict(path=baseline, version=baseline_version),
                   sourceSHA256=snapshot['sha256'], finalSourceSHA256=final_snapshot['sha256'],
                   sourceStable=snapshot['sha256'] == final_snapshot['sha256'])
    (out/'summary.json').write_text(json.dumps(summary, indent=2)+'\n')
    if pending:
        print('FEATURE STILL OPEN: ' + '; '.join(pending))
    if args.only:
        print('PARTIAL RUN: unselected checks were not executed')
    if not summary['sourceStable'] or any(r['status']=='fail' for r in results):
        return 1
    print('PASS: selected checks' if args.only or pending else 'PASS: all integration gates')
    return 2 if pending or args.only else 0


if __name__ == '__main__':
    sys.exit(main())
