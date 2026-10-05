#!/usr/bin/env python3
"""Run the native optional SQL pair against isolated PostgreSQL with two real drivers.

Opt in with GON_SQL_POSTGRES=1. Requires a running Docker server and the cached
official postgres:17-alpine image (override with GON_SQL_POSTGRES_IMAGE).
All database storage is disposable (a tmpfs-backed container); no existing
server or database is used. GON_SQL_GON may name another gon-compatible
executable, such as the private go of a source build, instead of gon/bin/gon.
"""

import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tempfile
import time
import uuid


ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / 'misc/gon/sqloptionalfixtures'
LOGS = ROOT / 'pkg/gon-validation/sql-optionals-postgres'
GON = Path(os.environ.get('GON_SQL_GON') or ROOT / 'gon/bin/gon')
EXPECTED = 'pgx PASS\npostgres PASS\n'
MODULE = '''module example.com/gon-sql-optionals-postgres

go 1.27

require (
    github.com/jackc/pgx/v5 v5.7.6
    github.com/lib/pq v1.10.9
)
'''


def main():
    if os.environ.get('GON_SQL_POSTGRES') != '1':
        print('SKIP: set GON_SQL_POSTGRES=1 to run disposable PostgreSQL integration')
        return
    baseline = os.environ.get('GON_BASELINE_GO')
    if not baseline or not Path(baseline).is_absolute() or not os.access(baseline, os.X_OK):
        raise SystemExit('Set GON_BASELINE_GO to an absolute unmodified Go 1.27+ executable')
    if not shutil.which('docker'):
        raise SystemExit('Docker is required for the optional PostgreSQL integration')
    LOGS.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    for name in ['GOROOT', 'GOTOOLDIR', 'GOWORK', 'GOOS', 'GOARCH']:
        env.pop(name, None)
    env.update(GOTOOLCHAIN='local', GOWORK='off', GOFLAGS='', GOMAXPROCS='2')
    summary = {
        'status': 'running',
        'host': {'system': platform.system(), 'machine': platform.machine()},
        'drivers': {'github.com/jackc/pgx/v5': 'v5.7.6', 'github.com/lib/pq': 'v1.10.9'},
        'checks': [],
    }

    def run(name, args, cwd=None, timeout=180, run_env=None):
        start = time.monotonic()
        result = subprocess.run([str(arg) for arg in args], cwd=cwd, env=run_env or env,
                                text=True, stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT, timeout=timeout)
        (LOGS / (name + '.log')).write_text(result.stdout)
        summary['checks'].append({'name': name, 'command': [str(arg) for arg in args],
                                  'exit_code': result.returncode,
                                  'duration_seconds': round(time.monotonic() - start, 3)})
        if result.returncode:
            raise RuntimeError(f'{name} failed; see {LOGS / (name + ".log")}\n{result.stdout}')
        return result.stdout

    name = 'gon-optionals-' + uuid.uuid4().hex[:12]
    started = False
    try:
        summary['baseline_version'] = run('baseline-version', [baseline, 'version']).strip()
        summary['gon_version'] = run('gon-version', [GON, 'version']).strip()
        summary['docker_version'] = run('docker-version', ['docker', 'version', '--format', '{{.Server.Version}}']).strip()
        image = os.environ.get('GON_SQL_POSTGRES_IMAGE', 'postgres:17-alpine')
        summary['postgres_image'] = image
        summary['postgres_image_id'] = run('postgres-image', ['docker', 'image', 'inspect', image, '--format', '{{.Id}}']).strip()
        run('postgres-start', ['docker', 'run', '--detach', '--rm', '--pull', 'never',
                              '--name', name, '--publish', '127.0.0.1::5432',
                              '--tmpfs', '/var/lib/postgresql/data:rw',
                              '--env', 'POSTGRES_USER=gon_optional_test',
                              '--env', 'POSTGRES_PASSWORD=gon_optional_test',
                              '--env', 'POSTGRES_DB=gon_optional_test', image])
        started = True
        deadline = time.monotonic() + 60
        while True:
            # Initialization uses a temporary socket-only server. Wait for the
            # final TCP listener, after creation of the requested database.
            ready = subprocess.run(['docker', 'exec', name, 'pg_isready', '-h', '127.0.0.1', '-U', 'gon_optional_test', '-d', 'gon_optional_test'],
                                   env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            if ready.returncode == 0:
                break
            if time.monotonic() >= deadline:
                raise RuntimeError('disposable PostgreSQL did not become ready within 60 seconds')
            time.sleep(0.25)
        ports = json.loads(run('postgres-port', ['docker', 'inspect', name, '--format', '{{json .NetworkSettings.Ports}}']))
        port = ports['5432/tcp'][0]['HostPort']
        summary['postgres_version'] = run('postgres-version', ['docker', 'exec', name, 'psql', '-U', 'gon_optional_test',
                                                              '-d', 'gon_optional_test', '-Atc', 'SHOW server_version']).strip()
        env['GON_SQL_TEST_DSN'] = f'postgres://gon_optional_test:gon_optional_test@127.0.0.1:{port}/gon_optional_test?sslmode=disable'
        with tempfile.TemporaryDirectory(prefix='gon-sql-postgres-') as temp:
            folder = Path(temp)
            (folder / 'go.mod').write_text(MODULE)
            shutil.copyfile(FIXTURES / 'common.go', folder / 'common.go')
            shutil.copyfile(FIXTURES / 'legacy.go', folder / 'optional.go')
            run('module-tidy', [baseline, 'mod', 'tidy'], cwd=folder)
            (LOGS / 'go.mod').write_text((folder / 'go.mod').read_text())
            (LOGS / 'go.sum').write_text((folder / 'go.sum').read_text())
            for check, tool in [('baseline-legacy', baseline), ('gon-legacy', GON)]:
                output = run(check, [tool, 'run', '.'], cwd=folder)
                if output != EXPECTED:
                    raise RuntimeError(f'{check}: unexpected observable results: {output!r}')
            shutil.copyfile(FIXTURES / 'modern.go', folder / 'optional.go')
            for check, flags in [('gon-modern', []), ('gon-modern-noinline', ['-gcflags=all=-l'])]:
                output = run(check, [GON, 'run', *flags, '.'], cwd=folder)
                if output != EXPECTED:
                    raise RuntimeError(f'{check}: unexpected observable results: {output!r}')
        summary['status'] = 'pass'
        print('PASS: PostgreSQL paired baseline/legacy/modern/noinline execution with pgx stdlib and lib/pq')
    except Exception as error:
        summary['status'] = 'fail'
        summary['error'] = str(error)
        raise
    finally:
        if started:
            result = subprocess.run(['docker', 'logs', name], env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            (LOGS / 'postgres.log').write_text(result.stdout)
            result = subprocess.run(['docker', 'rm', '--force', name], env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            summary['cleanup_exit_code'] = result.returncode
            (LOGS / 'postgres-cleanup.log').write_text(result.stdout)
            if result.returncode and summary['status'] == 'pass':
                summary['status'] = 'fail'
                summary['error'] = 'failed to remove the disposable PostgreSQL container'
        (LOGS / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    if summary['status'] != 'pass':
        raise SystemExit(summary['error'])


if __name__ == '__main__':
    main()
