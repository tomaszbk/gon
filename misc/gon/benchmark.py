#!/usr/bin/env python3
"""Compare upstream Go, Gon with identical Go source, and Gon with new syntax.

Only the small standalone fixture packages are built and executed. Results,
copied sources, commands, and raw measurements are retained under pkg/.
"""

import argparse
import datetime
import hashlib
import html
import json
import os
from pathlib import Path
import platform
import random
import re
import shlex
import shutil
import statistics
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / 'misc/gon/benchmarks/fixtures'
VARIANTS = ('go-legacy', 'gon-legacy', 'gon-modern')


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def distribution(values):
    return dict(median=statistics.median(values), min=min(values), max=max(values),
                samples=values)


def comparison(new, old):
    """Paired rounds, with a descriptive bootstrap interval for median ratios."""
    ratios = [a / b for a, b in zip(new, old)]
    rng = random.Random(927)
    boot = sorted(statistics.median(rng.choices(ratios, k=len(ratios)))
                  for _ in range(5000))
    return dict(medianPercent=(statistics.median(ratios) - 1) * 100,
                bootstrap95Percent=[(boot[125] - 1) * 100, (boot[4874] - 1) * 100])


def parse_benchmarks(output):
    result = {}
    for line in output.splitlines():
        if not line.startswith('Benchmark'):
            continue
        fields = line.split()
        if len(fields) < 4 or not fields[1].isdigit():
            continue
        name = re.sub(r'-\d+$', '', fields[0])
        if name in result:
            raise RuntimeError('duplicate benchmark: ' + name)
        result[name] = {fields[i + 1]: float(fields[i])
                        for i in range(2, len(fields) - 1, 2)}
    if not result:
        raise RuntimeError('no benchmark results')
    for name, values in result.items():
        if not {'ns/op', 'B/op', 'allocs/op'} <= values.keys():
            raise RuntimeError('incomplete benchmark: ' + name)
    return result


def parse_resources(output):
    if sys.platform == 'darwin':
        cpu = re.search(r'([\d.]+) real\s+([\d.]+) user\s+([\d.]+) sys', output)
        rss = re.search(r'(\d+)\s+maximum resident set size', output)
        if cpu and rss:
            return dict(userSeconds=float(cpu[2]), systemSeconds=float(cpu[3]),
                        maxRSSBytes=int(rss[1]))
    else:
        row = re.search(r'GON_RESOURCE ([\d.]+) ([\d.]+) (\d+)', output)
        if row:
            return dict(userSeconds=float(row[1]), systemSeconds=float(row[2]),
                        maxRSSBytes=int(row[3]) * 1024)
    raise RuntimeError('could not parse /usr/bin/time resource measurements')


def render_markdown(summary, path):
    """Keep a complete, linkable result table beside the machine-readable data."""
    lines = ['# Go / Gon benchmark results', '',
             f'Run: {summary["timestampUTC"]}; {summary["machine"]["system"]}; {summary["machine"]["cpu"]}.', '',
             f'{summary["samples"]} runtime samples per variant, {summary["benchtime"]} per benchmark/sample; '
             f'{summary["buildSamples"]} build samples. Positive percentages mean slower.', '',
             'Every operation processes 64 elements. All benchmarks are shown, including regressions.', '',
             '| Workload | Go legacy ns/op | Gon legacy ns/op | Gon modern ns/op | Modern / Gon legacy | B/op (Go / Gon / modern) | Allocs/op (Go / Gon / modern) |',
             '| --- | ---: | ---: | ---: | ---: | ---: | ---: |']
    for name, variants in summary['runtime'].items():
        ns = ' | '.join(f'{variants[v]["ns/op"]["median"]:,.1f}' for v in VARIANTS)
        memory = ' / '.join(f'{variants[v]["B/op"]["median"]:g}' for v in VARIANTS)
        allocs = ' / '.join(f'{variants[v]["allocs/op"]["median"]:g}' for v in VARIANTS)
        comp = summary['comparisons'][name]['modernVsGonLegacy']
        lo, hi = comp['bootstrap95Percent']
        lines.append(f'| {name.removeprefix("Benchmark")} | {ns} | {comp["medianPercent"]:+.1f}% '
                     f'[{lo:+.1f}%, {hi:+.1f}%] | {memory} | {allocs} |')
    lines += ['', '## Representation', '',
              'Sizes are bytes per value on this host. Retention is one untimed process sample per variant, '
              'with 10,000 present pointer payloads across a forced GC; heap deltas include runtime/allocator noise.', '',
              '| Shape | Go legacy bytes | Gon legacy bytes | Gon modern bytes |',
              '| --- | ---: | ---: | ---: |']
    for shape in sorted(summary['representation']['go-legacy']['sizesByShape']):
        sizes = ' | '.join(str(summary['representation'][v]['sizesByShape'][shape]) for v in VARIANTS)
        lines.append(f'| {shape} | {sizes} |')
    lines += ['', '| Variant | Slice storage bytes | Observed heap delta bytes | GC cycles | Checksum |',
              '| --- | ---: | ---: | ---: | ---: |']
    for variant in VARIANTS:
        retained = summary['representation'][variant]['retention']
        lines.append(f'| {variant} | {retained["sliceBytes"]} | {retained["heapDeltaBytes"]} | '
                     f'{retained["gcCycles"]} | {retained["checksum"]} |')
    lines += ['', '## Build', '',
              '| Variant | Rebuild median seconds | No-change median seconds | Rebuild CPU seconds | Reported RSS MiB | Executable bytes |',
              '| --- | ---: | ---: | ---: | ---: | ---: |']
    for variant in VARIANTS:
        build = summary['build'][variant]
        lines.append(f'| {variant} | {build["rebuild"]["wallSeconds"]["median"]:.3f} | '
                     f'{build["noop"]["wallSeconds"]["median"]:.3f} | '
                     f'{build["rebuild"]["cpuSeconds"]["median"]:.3f} | '
                     f'{build["rebuild"]["maxRSSBytes"]["median"] / 1048576:.1f} | '
                     f'{summary["artifacts"][variant]["appBytes"]} |')
    lines += ['', '## Reproduction and limits', '']
    for name, toolchain in summary['toolchains'].items():
        lines.append(f'- {name}: `{toolchain["version"]}`; compiler SHA-256 `{toolchain["compilerSHA256"]}`.')
    for name, checksum in summary['fixtureSHA256'].items():
        lines.append(f'- Fixture `{name}` SHA-256: `{checksum}`.')
    lines += ['', *['- ' + note for note in summary['methodology']], '',
              f'Correctness: {summary["correctness"]["testsPerVariant"]} tests per variant; identical stdout:', '',
              '```text', summary['correctness']['stdout'].strip(), '```', '',
              'Raw evidence: [runtime samples](runtime-raw.json), [build samples](build-raw.json), '
              '[representation](representation.json), [summary](summary.json), [commands](commands.json), '
              '[HTML](report.html). Source copies and logs are in the same run directory.', '']
    path.write_text('\n'.join(lines))


def render_report(summary, path):
    esc = html.escape
    rows = []
    for name, variants in summary['runtime'].items():
        cells = []
        for variant in VARIANTS:
            values = variants[variant]
            ns = values['ns/op']
            cells.append(f'<td>{ns["median"]:,.1f}<small>{ns["min"]:,.1f}–{ns["max"]:,.1f}</small></td>')
        comp = summary['comparisons'][name]['modernVsGonLegacy']
        lo, hi = comp['bootstrap95Percent']
        memory = ' / '.join(f'{variants[v]["B/op"]["median"]:g} B, '
                            f'{variants[v]["allocs/op"]["median"]:g} alloc' for v in VARIANTS)
        rows.append(f'<tr><th>{esc(name.removeprefix("Benchmark"))}</th>{"".join(cells)}'
                    f'<td>{comp["medianPercent"]:+.1f}%<small>95%: {lo:+.1f}%…{hi:+.1f}%</small></td>'
                    f'<td>{memory}</td></tr>')
    build_rows = []
    for variant in VARIANTS:
        data = summary['build'][variant]
        rebuild = data['rebuild']['wallSeconds']
        noop = data['noop']['wallSeconds']
        build_rows.append(f'<tr><th>{variant}</th><td>{rebuild["median"]:.3f} s'
                          f'<small>{rebuild["min"]:.3f}–{rebuild["max"]:.3f}</small></td>'
                          f'<td>{noop["median"]:.3f} s<small>{noop["min"]:.3f}–{noop["max"]:.3f}</small></td>'
                          f'<td>{data["rebuild"]["cpuSeconds"]["median"]:.3f} s</td>'
                          f'<td>{data["rebuild"]["maxRSSBytes"]["median"] / 1048576:.1f} MiB</td>'
                          f'<td>{summary["artifacts"][variant]["appBytes"]:,} B</td></tr>')
    notes = ''.join(f'<li>{esc(n)}</li>' for n in summary['methodology'])
    observations = ''.join(f'<li>{esc(n)}</li>' for n in summary.get('observations', []))
    if observations:
        observations = '<h2>Hallazgos de esta ejecución</h2><ul>' + observations + '</ul>'
    if summary.get('diagnostics'):
        observations += '<p>Evidencia del compilador: <a href="diagnostics/diagnostics.json">inlining y ensamblado</a>.</p>'
    versions = '<br>'.join(f'<b>{esc(k)}:</b> {esc(v["version"])}'
                          for k, v in summary['toolchains'].items())
    path.write_text(f'''<!doctype html><html lang="es"><meta charset="utf-8">
<title>Go / Gon benchmark</title><style>
body{{font:15px system-ui;max-width:1400px;margin:40px auto;padding:0 24px;color:#182638;background:#f8fafc}}
h1{{font-size:28px}}table{{border-collapse:collapse;background:white;width:100%;margin:20px 0}}
th,td{{padding:12px;text-align:right;border-bottom:1px solid #d9e2ec}}th:first-child{{text-align:left}}
small{{display:block;color:#64748b;font-size:11px}}li{{margin:7px 0}}code{{font-size:12px}}
</style><h1>Go / Gon: ejecución y compilación</h1><p>{versions}</p>
<p>{esc(summary['machine']['system'])} · {esc(summary['machine']['cpu'])} ·
{summary['samples']} muestras por variante · {esc(summary['benchtime'])} por benchmark/muestra.</p>
<p>Tiempo por operación: mediana en ns/op; debajo, mínimo–máximo.
Una operación procesa un lote de 64 elementos. Un porcentaje positivo significa más lento.</p>
<table><tr><th>Escenario</th><th>Go clásico</th><th>Gon clásico</th><th>Gon moderno</th>
<th>Moderno vs Gon clásico</th><th>B/op y allocs/op (Go / Gon / moderno)</th></tr>{''.join(rows)}</table>
<h2>Compilación y tamaño</h2><table><tr><th>Variante</th><th>Recompilar paquete</th>
<th>Build sin cambios</th><th>CPU recompilación</th><th>RSS informado</th><th>Ejecutable</th></tr>
{''.join(build_rows)}</table><h2>Metodología y límites</h2><ul>{notes}</ul>
{observations}<p>Pruebas: {summary['correctness']['testsPerVariant']} por variante; stdout idéntico:
<code>{esc(summary['correctness']['stdout'].strip())}</code></p>
<p>Datos completos: <a href="summary.json">summary.json</a>; <a href="report.md">tabla Markdown y representación</a>.
Fuentes y logs en este mismo directorio.</p>
</html>''')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default=os.environ.get('GON_BASELINE_GO'), help='absolute path to unmodified Go')
    parser.add_argument('--gon', default=str(ROOT / 'gon/bin/gon'))
    parser.add_argument('--samples', type=int, default=10)
    parser.add_argument('--build-samples', type=int, default=7)
    parser.add_argument('--benchtime', default='300ms')
    parser.add_argument('--seed', type=int, default=927)
    parser.add_argument('--output', type=Path, help='new output directory (must not already exist)')
    parser.add_argument('--correctness-only', action='store_true',
                        help='build, test and compare outputs/representation without running timed benchmarks or build samples')
    args = parser.parse_args()
    if not args.go or not Path(args.go).is_absolute() or not os.access(args.go, os.X_OK):
        parser.error('set GON_BASELINE_GO or --go to an absolute unmodified Go executable')
    if args.samples < 2 or args.build_samples < 2:
        parser.error('at least two samples required')
    if sys.platform not in ('darwin', 'linux'):
        parser.error('resource measurement currently supports macOS and Linux')
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    out = (args.output or ROOT / 'pkg/gon-benchmarks' / stamp).resolve()
    out.mkdir(parents=True, exist_ok=False)
    (out / 'logs').mkdir()
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(('GO', 'GON', 'CGO'))}
    env.update(GOENV='off', GOWORK='off', GOTOOLCHAIN='local', CGO_ENABLED='0',
               GOMAXPROCS='1', GOFLAGS='', GOGC='100', GOMEMLIMIT='off', LC_ALL='C')
    commands = []

    def run(command, cwd, label, run_env, measured=False, allow_failure=False):
        original = [str(a) for a in command]
        command = original
        if measured:
            flags = ['-l'] if sys.platform == 'darwin' else ['-f', 'GON_RESOURCE %U %S %M']
            command = ['/usr/bin/time', *flags, *original]
        start = time.perf_counter()
        proc = subprocess.run(command, cwd=cwd, env=run_env, text=True,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        elapsed = time.perf_counter() - start
        (out / 'logs' / (label + '.log')).write_text(proc.stdout + proc.stderr)
        commands.append(dict(label=label, command=command, cwd=str(cwd),
                             cache=run_env.get('GOCACHE'), exitCode=proc.returncode,
                             wallSeconds=elapsed))
        (out / 'commands.json').write_text(json.dumps(commands, indent=2) + '\n')
        if proc.returncode and not allow_failure:
            raise RuntimeError(f'{label}: {shlex.join(original)}\n{proc.stdout}{proc.stderr}')
        metrics = dict(wallSeconds=elapsed)
        if allow_failure:
            metrics['exitCode'] = proc.returncode
        if measured:
            metrics.update(parse_resources(proc.stderr))
            metrics['cpuSeconds'] = metrics['userSeconds'] + metrics['systemSeconds']
        return proc.stdout, metrics

    toolchains = {}
    for name, executable in [('go', args.go), ('gon', args.gon)]:
        version, _ = run([executable, 'version'], ROOT, name + '-version', env)
        details, _ = run([executable, 'env', '-json', 'GOOS', 'GOARCH', 'GOARM64',
                          'GOAMD64', 'GOROOT', 'GOVERSION', 'GOEXPERIMENT'], ROOT, name + '-env', env)
        details = json.loads(details)
        if name == 'go':
            released = re.fullmatch(r'go1\.(\d+)(?:\.\d+)?', details['GOVERSION'])
            if released is None or int(released[1]) < 27:
                raise RuntimeError('baseline must be released unmodified Go 1.27+: ' + details['GOVERSION'])
            _, probe = run([executable, 'capabilities', '--json'], ROOT,
                           'go-gon-capabilities-probe', env, allow_failure=True)
            if probe['exitCode'] == 0:
                raise RuntimeError('baseline accepts Gon capabilities; select an unmodified Go executable')
        compiler = Path(details['GOROOT']) / 'pkg/tool' / (details['GOOS'] + '_' + details['GOARCH']) / 'compile'
        toolchains[name] = dict(executable=str(Path(executable).resolve()), version=version.strip(),
                                environment=details, compilerSHA256=digest(compiler))
    if toolchains['go']['environment']['GOROOT'] == toolchains['gon']['environment']['GOROOT']:
        raise RuntimeError('baseline Go must not be the Gon toolchain')
    if any(toolchains['go']['environment'][k] != toolchains['gon']['environment'][k]
           for k in ('GOOS', 'GOARCH')):
        raise RuntimeError('toolchain platforms differ')
    print('OUTPUT ' + str(out), flush=True)
    print('Preparing caches and checking all three variants...', flush=True)
    machine = dict(system=platform.platform(), cpu=platform.processor(), logicalCPUs=os.cpu_count())
    if sys.platform == 'darwin':
        machine['cpu'] = subprocess.check_output(['sysctl', '-n', 'machdep.cpu.brand_string'], text=True).strip()
        machine['memoryBytes'] = int(subprocess.check_output(['sysctl', '-n', 'hw.memsize'], text=True))
    head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    dirty = bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True))
    fixture_sha = {name: digest(FIXTURES / name) for name in
                   ('common.go', 'common_test.go', 'legacy.go', 'modern.go')}
    configs, artifacts, outputs, tests, representation = {}, {}, {}, {}, {}
    for variant in VARIANTS:
        directory = out / variant
        directory.mkdir()
        source = 'modern.go' if variant == 'gon-modern' else 'legacy.go'
        for filename in ('common.go', 'common_test.go'):
            shutil.copyfile(FIXTURES / filename, directory / filename)
        shutil.copyfile(FIXTURES / source, directory / 'impl.go')
        (directory / 'go.mod').write_text('module example.com/gonbench\n\ngo 1.26\n')
        (directory / 'nonce.go').write_text('package main\nconst buildNonce = 0\n')
        executable = args.go if variant == 'go-legacy' else args.gon
        cache = out / ('cache-go' if variant == 'go-legacy' else 'cache-gon')
        variant_env = dict(env, GOCACHE=str(cache))
        configs[variant] = (directory, executable, variant_env)
        run([executable, 'test', '-c', '-vet=off', '-p=1', '-trimpath', '-buildvcs=false',
             '-o', directory / 'bench.test', '.'], directory, variant + '-test-build', variant_env)
        output, _ = run([directory / 'bench.test', '-test.run=^Test', '-test.v', '-test.count=1'],
                        directory, variant + '-correctness', variant_env)
        tests[variant] = len(re.findall(r'^--- PASS: Test', output, re.MULTILINE))
        if not tests[variant] or '\nPASS\n' not in '\n' + output:
            raise RuntimeError('no passing correctness tests for ' + variant)
        run([executable, 'build', '-p=1', '-trimpath', '-buildvcs=false', '-o', directory / 'app', '.'],
            directory, variant + '-app-warmup', variant_env)
        outputs[variant], _ = run([directory / 'app'], directory, variant + '-app-output', variant_env)
        output, _ = run([directory / 'app', '--representation'], directory,
                        variant + '-representation', variant_env)
        representation[variant] = json.loads(output)
        retained = representation[variant]['retention']
        if retained['count'] != 10000 or retained['checksum'] != 49995000 or retained['sliceBytes'] <= 0:
            raise RuntimeError('invalid retained payload representation for ' + variant)
        artifacts[variant] = dict(appBytes=(directory / 'app').stat().st_size,
                                  testBytes=(directory / 'bench.test').stat().st_size,
                                  implementationLines=len((directory / 'impl.go').read_text().splitlines()),
                                  implementationBytes=(directory / 'impl.go').stat().st_size,
                                  sourceSHA256={p: digest(directory / p) for p in
                                                ('common.go', 'common_test.go', 'impl.go', 'go.mod', 'nonce.go')})
    if len(set(outputs.values())) != 1 or len(set(tests.values())) != 1:
        raise RuntimeError('variants differ in output or executed test count')
    if artifacts['go-legacy']['sourceSHA256'] != artifacts['gon-legacy']['sourceSHA256']:
        raise RuntimeError('legacy source differs between toolchains')
    shape_sets = [set(representation[v]['sizesByShape']) for v in VARIANTS]
    if any(shapes != shape_sets[0] for shapes in shape_sets):
        raise RuntimeError('representation shape sets differ')
    (out / 'representation.json').write_text(json.dumps(representation, indent=2) + '\n')
    print(f'PASS: {tests["go-legacy"]} tests per variant; identical program output.', flush=True)
    evidence = dict(schemaVersion=2, timestampUTC=stamp, repositoryHEAD=head, repositoryDirty=dirty,
                    integratedUpstream=json.loads((ROOT / 'misc/gon/UPSTREAM.json').read_text()),
                    toolchains=toolchains, machine=machine, fixtureSHA256=fixture_sha, artifacts=artifacts,
                    correctness=dict(testsPerVariant=tests['go-legacy'], stdout=outputs['go-legacy']),
                    representation=representation, timedMeasurements=False)
    (out / 'correctness.json').write_text(json.dumps(evidence, indent=2) + '\n')
    if args.correctness_only:
        print('Correctness evidence: ' + str(out / 'correctness.json'), flush=True)
        return 0
    rng = random.Random(args.seed)
    raw_runtime = {v: [] for v in VARIANTS}
    raw_build = {v: dict(rebuild=[], noop=[]) for v in VARIANTS}
    # Warm every benchmark before taking measurements. Test setup and process
    # startup are outside the testing package's reported ns/op.
    for variant in VARIANTS:
        directory, _, variant_env = configs[variant]
        run([directory / 'bench.test', '-test.run=^$', '-test.bench=.', '-test.benchmem',
             '-test.benchtime=100ms', '-test.count=1', '-test.cpu=1'], directory,
            variant + '-bench-warmup', variant_env)
    for round_number in range(args.samples):
        order = rng.sample(list(VARIANTS), len(VARIANTS))
        print(f'Runtime round {round_number + 1}/{args.samples}: ' + ', '.join(order), flush=True)
        for variant in order:
            directory, _, variant_env = configs[variant]
            output, _ = run([directory / 'bench.test', '-test.run=^$', '-test.bench=.', '-test.benchmem',
                             '-test.benchtime=' + args.benchtime, '-test.count=1', '-test.cpu=1'],
                            directory, f'{variant}-runtime-{round_number:02d}', variant_env)
            raw_runtime[variant].append(parse_benchmarks(output))
        (out / 'runtime-raw.json').write_text(json.dumps(raw_runtime, indent=2) + '\n')
    for round_number in range(args.build_samples):
        order = rng.sample(list(VARIANTS), len(VARIANTS))
        print(f'Build round {round_number + 1}/{args.build_samples}', flush=True)
        for variant in order:
            directory, executable, variant_env = configs[variant]
            # Same observable constant change in every variant. main prints it,
            # forcing compile+link while dependencies stay hot.
            (directory / 'nonce.go').write_text(f'package main\nconst buildNonce = {round_number + 1}\n')
            for mode in ('rebuild', 'noop'):
                _, metrics = run([executable, 'build', '-p=1', '-trimpath', '-buildvcs=false',
                                  '-o', directory / 'app', '.'], directory,
                                 f'{variant}-{mode}-{round_number:02d}', variant_env, measured=True)
                raw_build[variant][mode].append(metrics)
        (out / 'build-raw.json').write_text(json.dumps(raw_build, indent=2) + '\n')
    # Leave the retained sources and app exactly in the initial state reported
    # in correctness/artifacts. Restoration is outside every timed sample.
    for variant in VARIANTS:
        directory, executable, variant_env = configs[variant]
        (directory / 'nonce.go').write_text('package main\nconst buildNonce = 0\n')
        run([executable, 'build', '-p=1', '-trimpath', '-buildvcs=false', '-o', directory / 'app', '.'],
            directory, variant + '-app-restore', variant_env)
        restored, _ = run([directory / 'app'], directory, variant + '-restored-output', variant_env)
        if restored != outputs[variant] or (directory / 'app').stat().st_size != artifacts[variant]['appBytes']:
            raise RuntimeError('restored artifact differs for ' + variant)
    expected_names = set(raw_runtime['go-legacy'][0])
    if any(set(sample) != expected_names for samples in raw_runtime.values() for sample in samples):
        raise RuntimeError('benchmark sets differ')
    runtime, comparisons = {}, {}
    for name in sorted(expected_names):
        runtime[name] = {v: {metric: distribution([s[name][metric] for s in raw_runtime[v]])
                             for metric in ('ns/op', 'B/op', 'allocs/op')} for v in VARIANTS}
        comparisons[name] = {
            'gonLegacyVsGo': comparison(runtime[name]['gon-legacy']['ns/op']['samples'],
                                       runtime[name]['go-legacy']['ns/op']['samples']),
            'modernVsGonLegacy': comparison(runtime[name]['gon-modern']['ns/op']['samples'],
                                           runtime[name]['gon-legacy']['ns/op']['samples'])}
    build = {v: {mode: {metric: distribution([s[metric] for s in raw_build[v][mode]])
                       for metric in raw_build[v][mode][0]} for mode in ('rebuild', 'noop')}
             for v in VARIANTS}
    for name, toolchain in toolchains.items():
        details = toolchain['environment']
        compiler = Path(details['GOROOT']) / 'pkg/tool' / (details['GOOS'] + '_' + details['GOARCH']) / 'compile'
        if digest(compiler) != toolchain['compilerSHA256']:
            raise RuntimeError(name + ' compiler changed during the run; retained samples are invalid')
    if any(digest(FIXTURES / name) != checksum for name, checksum in fixture_sha.items()):
        raise RuntimeError('fixtures changed during the run; retained samples are invalid')
    summary = dict(schemaVersion=2, timestampUTC=stamp, repositoryHEAD=head, repositoryDirty=dirty,
                   integratedUpstream=json.loads((ROOT / 'misc/gon/UPSTREAM.json').read_text()),
                   machine=machine, toolchains=toolchains, samples=args.samples, buildSamples=args.build_samples,
                   benchtime=args.benchtime, seed=args.seed, environmentOverrides={k: env[k] for k in
                   ('GOENV', 'GOWORK', 'GOTOOLCHAIN', 'CGO_ENABLED', 'GOMAXPROCS', 'GOFLAGS', 'GOGC', 'GOMEMLIMIT')},
                   correctness=dict(testsPerVariant=tests['go-legacy'], stdout=outputs['go-legacy']),
                   artifacts=artifacts, fixtureSHA256=fixture_sha, representation=representation,
                   runtime=runtime, comparisons=comparisons, build=build,
                   methodology=[
                       'Go clásico y Gon clásico usan fuentes idénticas (SHA-256 comprobado). Las tres variantes comparten datos, pruebas y benchmarks.',
                       'Go y Gon tienen versiones base distintas: esa comparación mezcla cambios upstream, del runtime y de Gon. Gon moderno vs Gon clásico aísla la sintaxis en el mismo toolchain.',
                       'Ejecución serial, orden de variantes aleatorio en cada ronda, GOMAXPROCS=1, CGO desactivado, optimizaciones por defecto; no se desactiva inlining.',
                       'Cada ns/op, B/op y allocs/op corresponde a un lote de 64 elementos. Preparación de datos fuera del cronómetro; sumas consumidas por sinks globales. Contadores compartidos hacen observable la evaluación y agregan costo de instrumentación.',
                       'Las asignaciones son bytes/objetos de heap por operación, no memoria total del proceso. Los buffers reutilizados no cuentan como asignaciones nuevas.',
                       'Representation: tamaños de valor por unsafe.Sizeof y una muestra no temporizada de retención por variante, con 10.000 payloads puntero y un GC forzado. El delta de heap incluye ruido; no es un presupuesto ni una comparación estadística.',
                       'Enums y opcionales nativos se comparan con structs tagged Go explícitos que distinguen presencia por tag, incluidos payloads nil presentes.',
                       'Recompilación: dependencias calientes, cambio del entero buildNonce impreso por main que invalida el paquete principal y el enlace, compile+link con -p=1 y -trimpath. No mide construir el toolchain ni una compilación con toda la caché vacía.',
                       'Build sin cambios usa la caché y el ejecutable existente. Wall time incluye el comando público y, en Gon, su launcher. CPU es user+system informado por /usr/bin/time.',
                       'RSS es el máximo informado por /usr/bin/time, no la suma simultánea de memoria de todos los subprocesos. Ejecutables sin stripping; incluyen runtime y metadatos de depuración.',
                       'Intervalos 95%: bootstrap descriptivo de medianas de ratios por ronda (5000 remuestreos). No corrigen múltiples comparaciones ni garantizan generalización.',
                       'Resultados de una sola máquina/sesión, sin fijar núcleo ni controlar frecuencia, carga externa o estado térmico. No prueban rendimiento en otras arquitecturas o aplicaciones.',
                   ])
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    render_report(summary, out / 'report.html')
    render_markdown(summary, out / 'report.md')
    print('\nns/op median (Go legacy | Gon legacy | Gon modern):', flush=True)
    for name in runtime:
        print(name + ': ' + ' | '.join(f'{runtime[name][v]["ns/op"]["median"]:,.1f}' for v in VARIANTS))
    print('\nReport: ' + str(out / 'report.html'))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, RuntimeError, ValueError) as error:
        print('FAIL: ' + str(error), file=sys.stderr)
        sys.exit(1)
