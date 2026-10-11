#!/usr/bin/env python3
"""Execute Go/Gon module-version pairs and module/editor command regressions."""
import json
import os
from pathlib import Path
import shutil
import re
import zipfile
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
GON = ROOT / 'gon/bin/gon'
BASELINE = os.environ.get('GON_BASELINE_GO')
ENV = {k: v for k, v in os.environ.items() if not k.startswith(('GO', 'GON', 'CGO'))}
ENV.update(GOENV='off', GOWORK='off', GOTOOLCHAIN='local', GOFLAGS='')
checks = 0


def run(folder, *args, tool=GON, error=None, env=ENV):
    global checks
    result = subprocess.run([str(tool), *args], cwd=folder, env=env, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    output = result.stdout + result.stderr
    if error is None:
        assert result.returncode == 0, f'{args}: {output}'
    else:
        assert result.returncode != 0 and error in output, f'{args}: {output}'
    checks += 1
    return result.stdout


def write(folder, name, text):
    target = folder / name
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text)


def main():
    assert BASELINE and Path(BASELINE).is_absolute(), 'set GON_BASELINE_GO to unmodified Go 1.27+'
    baseline_version = run(ROOT, 'version', tool=BASELINE)
    match = re.search(r'go1\.(\d+)\.', baseline_version)
    assert match and int(match[1]) >= 27, baseline_version
    expected = 'PASS: errors, nil, optionals, enums, matching, lambdas, conditionals, named arguments\n'
    with tempfile.TemporaryDirectory(prefix='gon-moduleversions-') as temporary:
        root = Path(temporary).resolve()
        # Same asserted runtime behavior on unmodified Go and both Gon dialects.
        for variant, version, tool in [('legacy', '1.27', BASELINE), ('legacy', '1.27', GON),
                                      ('modern', '2.27', GON), ('modern', '2.27.0', GON)]:
            folder = root / (variant + version + Path(tool).name)
            folder.mkdir()
            write(folder, 'go.mod', f'module example.com/pair\n\ngo {version}\n')
            for name in ['common', variant]:
                shutil.copyfile(ROOT / 'misc/gon/readme-examples' / (name + '.go'), folder / (name + '.go'))
            assert run(folder, 'run', '.', tool=tool) == expected
            if version == '2.27':
                run(folder, 'build', '-o', 'pair', '.')
                run(folder, 'test', './...')
                run(folder, 'vet', './...')
                run(folder, 'fmt', './...')
                run(folder, 'check', './...')
                run(folder, 'mod', 'tidy', '-go=2.27')
                assert 'go 2.27\n' in (folder / 'go.mod').read_text()
                assert json.loads(run(folder, 'list', '-m', '-json'))['GoVersion'] == '2.27'
                run(folder, 'list', '.', tool=BASELINE, error='requires go >= 2.27')
                run(folder, 'mod', 'graph', '-go=2.27')
        # Cgo compilation uses the same module version and keeps C behavior.
        for version, tool in [('1.27', BASELINE), ('1.27', GON), ('2.27', GON)]:
            cgo = root / ('cgo-' + version + Path(tool).name)
            write(cgo, 'go.mod', f'module example.com/cgopair\n\ngo {version}\n')
            write(cgo, 'main.go', 'package main\n/* static int twice(int value) { return value * 2; } */\nimport "C"\nimport "fmt"\nfunc main() { fmt.Println(C.twice(3)) }\n')
            assert run(cgo, 'run', '.', tool=tool, env=dict(ENV, CGO_ENABLED='1')) == '6\n'
        # Exported native optional types, a Gon dependency, a Go dependency,
        # module graph updates, vendoring and workspace version requirements.
        lib, app, old = root / 'lib', root / 'app', root / 'old'
        write(lib, 'go.mod', 'module example.com/lib\n\ngo 2.27\n')
        write(lib, 'lib.go', 'package lib\nfunc Twice(value int?) int? { return (value ?? 0) * 2 }\n')
        write(old, 'go.mod', 'module example.com/old\n\ngo 1.27\n')
        write(old, 'old.go', 'package old\nfunc Three() int { return 3 }\n')
        write(app, 'go.mod', 'module example.com/app\n\ngo 2.27\nrequire (\nexample.com/lib v0.0.0\nexample.com/old v0.0.0\n)\nreplace example.com/lib => ../lib\nreplace example.com/old => ../old\n')
        write(app, 'main.go', 'package main\nimport ("fmt"; "example.com/lib"; "example.com/old")\nfunc main() { fmt.Println(lib.Twice(value: old.Three()) ?? 0) }\n')
        write(app, 'main_test.go', 'package main\nimport ("testing"; "example.com/lib")\nfunc TestTwice(t *testing.T) { if got := lib.Twice(value: 3) ?? 0; got != 6 { t.Fatal(got) } }\n')
        assert run(app, 'run', '.') == '6\n'
        run(app, 'test', '-cover', './...')
        run(app, 'vet', './...')
        run(app, 'check', './...')
        run(app, 'mod', 'tidy')
        run(app, 'mod', 'vendor')
        assert run(app, 'run', '-mod=vendor', '.') == '6\n'
        run(app, 'test', '-mod=vendor', './...')
        # A Go module consuming a Gon dependency gains the Gon requirement.
        s = (app / 'go.mod').read_text().replace('go 2.27', 'go 1.27')
        (app / 'go.mod').write_text(s)
        run(app, 'mod', 'tidy')
        assert 'go 2.27\n' in (app / 'go.mod').read_text()
        assert run(app, 'run', '-mod=mod', '.') == '6\n'
        write(root, 'go.work', 'go 2.27\nuse (\n./app\n./lib\n./old\n)\n')
        workspace = dict(ENV, GOWORK=str(root / 'go.work'))
        assert run(app, 'run', '.', env=workspace) == '6\n'
        run(root, 'work', 'sync', env=workspace)
        write(root, 'go.work', 'go 2.28\nuse ./app\n')
        run(app, 'list', '.', env=workspace, error='requires Gon >= 2.28')
        # Fetch a Gon module from a local module proxy, not just a replace.
        proxy = root / 'proxy/example.com/downloaded/@v'
        for version, directive in [('v0.1.0', '2.27'), ('v0.2.0', '2.28')]:
            mod = f'module example.com/downloaded\n\ngo {directive}\n'
            write(proxy, version + '.mod', mod)
            write(proxy, version + '.info', json.dumps({'Version': version, 'Time': '2026-10-11T00:00:00Z'}))
            with zipfile.ZipFile(proxy / (version + '.zip'), 'w') as archive:
                archive.writestr('example.com/downloaded@' + version + '/go.mod', mod)
                archive.writestr('example.com/downloaded@' + version + '/lib.go', 'package downloaded\nfunc Value() int? { return 6 }\n')
        downloaded = root / 'downloaded-app'
        write(downloaded, 'go.mod', 'module example.com/downloadapp\n\ngo 2.27\nrequire example.com/downloaded v0.1.0\n')
        write(downloaded, 'main.go', 'package main\nimport ("fmt"; "example.com/downloaded")\nfunc main() { fmt.Println(downloaded.Value() ?? 0) }\n')
        proxy_env = dict(ENV, GOPROXY=(root / 'proxy').as_uri(), GOSUMDB='off', GOMODCACHE=str(root / 'modcache'))
        run(downloaded, 'mod', 'download', 'all', env=proxy_env)
        assert run(downloaded, 'run', '.', env=proxy_env) == '6\n'
        run(downloaded, 'mod', 'download', 'example.com/downloaded@v0.2.0', env=proxy_env, error='requires Gon >= 2.28')
        auto = dict(ENV, GOTOOLCHAIN='auto', GOROOT=str(ROOT))
        assert run(app, 'run', '-mod=mod', '.', tool=ROOT / 'bin/go', env=auto) == '6\n'
        run(app, 'get', 'go@2.27')
        # Future releases are rejected using Gon diagnostics, including patches.
        bad = root / 'bad'
        write(bad, 'main.go', 'package main\nfunc main() {}\n')
        for version in ['2.27.1', '2.28', '3.0']:
            write(bad, 'go.mod', f'module example.com/bad\n\ngo {version}\n')
            run(bad, 'build', '.', error=f'requires Gon >= {version}')
            run(bad, 'list', '.', tool=ROOT / 'bin/go', env=auto, error=f'requires Gon >= {version}')
        write(bad, 'go.mod', 'module example.com/bad\n\ngo 1.999\n')
        run(bad, 'build', '.', error='requires go >= 1.999')
        write(bad, 'go.mod', 'module example.com/bad\n\ngo 2.027\n')
        run(bad, 'build', '.', error='invalid go version')
        run(bad, 'tool', 'compile', '-lang=go2.28', 'main.go', error='requires Gon >= 2.28')
    print(f'PASS: {checks} module-version checks; Go/Gon executable pairs and native host only')


if __name__ == '__main__':
    main()
