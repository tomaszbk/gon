from pathlib import Path
import ctypes, datetime, hashlib, json, os, shutil, subprocess, tempfile, time
root = Path('/Users/tzbk/Documents/gon')
out = root / 'pkg/gon-native-validation'
out.mkdir(parents=True, exist_ok=True)
temp = Path(tempfile.mkdtemp(prefix='gon-performance-bootstrap-'))
snapshot = temp / 'gon'
snapshot.mkdir()
baseline = '/opt/homebrew/bin/go'
bootstrap = '/opt/homebrew/Cellar/go/1.27.1/libexec'
cache = temp / 'gocache'
cache.mkdir()
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
libsystem = ctypes.CDLL('/usr/lib/libSystem.B.dylib', use_errno=True)
libsystem.clonefile.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
libsystem.clonefile.restype = ctypes.c_int
def clone_source(src, dest):
    if src.is_symlink():
        shutil.copy2(src, dest, follow_symlinks=False)
        return
    if libsystem.clonefile(os.fsencode(src), os.fsencode(dest), 0) != 0:
        number = ctypes.get_errno()
        raise OSError(number, os.strerror(number), str(src))
start = time.monotonic()
report = {'snapshot':str(snapshot), 'temporaryRoot':str(temp), 'command':['./make.bash'], 'cwd':str(snapshot / 'src'), 'baseline':baseline, 'baselineRoot':bootstrap, 'status':'running', 'startedAt':now(), 'freshGOCACHE':str(cache), 'temporarySnapshotRemoved':False, 'checks':[], 'snapshotPolicy':'Tracked plus maintained untracked source files; excludes Git metadata, root tool binaries, gon distribution, pkg, local design/handover/AGENTS documents, unrelated untracked top-level examples/media and benchmark outputs. VERSION copied from VERSION.cache. Fresh temporary GOCACHE. APFS clonefile copy-on-write snapshot, no hardlinks.'}
jsonpath = out / 'bootstrap.json'
def save():
    jsonpath.write_text(json.dumps(report, indent=2)+'\n')
def invoke(name, command, cwd, env):
    path = out / ('bootstrap.log' if name == 'make' else 'bootstrap.'+name+'.log')
    t = time.monotonic()
    print('Starting '+name, flush=True)
    with path.open('w') as log:
        result = subprocess.run(command, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT)
    check = {'name':name, 'command':command, 'status':'pass' if result.returncode == 0 else 'fail', 'exitCode':result.returncode, 'seconds':round(time.monotonic()-t, 3), 'log':str(path)}
    print(name+': '+check['status']+' ('+str(check['seconds'])+'s)', flush=True)
    return check
try:
    report['baselineVersion'] = subprocess.check_output([baseline,'version'],text=True).strip()
    tracked = set(subprocess.check_output(['git','ls-files','-z'],cwd=root).decode().rstrip('\0').split('\0'))
    untracked = set(subprocess.check_output(['git','ls-files','--others','--exclude-standard','-z'],cwd=root).decode().rstrip('\0').split('\0'))
    def wanted(p, is_tracked):
        if not p or p == 'AGENTS.md' or p.startswith(('.git/', 'bin/', 'pkg/', 'gon/', 'design/', 'handover/', '.agents/', 'misc/gon/benchmarks/results/')):
            return False
        return is_tracked or p.startswith(('src/', 'test/', 'tools/', 'misc/', 'doc/', 'api/'))
    files = sorted(p for p in tracked | untracked if wanted(p, p in tracked) and (root / p).is_file())
    manifest = []
    for p in files:
        src, dest = root / p, snapshot / p
        dest.parent.mkdir(parents=True, exist_ok=True)
        clone_source(src, dest)
        manifest.append(hashlib.sha256(dest.read_bytes()).hexdigest()+'  '+p)
    (snapshot / 'VERSION').write_bytes((root / 'VERSION.cache').read_bytes())
    if 'VERSION' not in files:
        files.append('VERSION')
    manifest = [entry for entry in manifest if not entry.endswith('  VERSION')]
    manifest.append(hashlib.sha256((snapshot / 'VERSION').read_bytes()).hexdigest()+'  VERSION')
    sourcepaths = out / 'bootstrap.source-files.txt'
    sourcepaths.write_text('\n'.join(sorted(files))+'\n')
    sourcehashes = out / 'bootstrap.sha256-manifest.txt'
    sourcehashes.write_text('\n'.join(sorted(manifest))+'\n')
    report.update(sourceFileCount=len(files), sourceFileManifest=str(sourcepaths), sourceSha256Manifest=str(sourcehashes), sourceManifestSha256=hashlib.sha256(sourcehashes.read_bytes()).hexdigest(), sourcePathsSha256=hashlib.sha256(sourcepaths.read_bytes()).hexdigest())
    save()
    env = {k:v for k,v in os.environ.items() if k not in ('GOROOT','GOTOOLDIR','GOFLAGS')}
    env.update(GOROOT_BOOTSTRAP=bootstrap, GOCACHE=str(cache), GOENV='off', GOTOOLCHAIN='local', GOFLAGS='-p=2', GOMAXPROCS='2')
    report['environment'] = {'GOCACHE':str(cache), 'GOROOT_BOOTSTRAP':bootstrap, 'GOFLAGS':'-p=2', 'GOMAXPROCS':'2'}
    save()
    make = invoke('make', ['./make.bash'], snapshot / 'src', env)
    report.update(status=make['status'], exitCode=make['exitCode'], seconds=make['seconds'], log=make['log'])
    save()
    if make['exitCode'] != 0:
        raise RuntimeError('source bootstrap failed')
    tool = str(snapshot / 'bin/go')
    env.update(GON_BASELINE_GO=baseline)
    check = invoke('optionresult-core-and-boundaries', [tool,'run',str(snapshot / 'test/optionresult.go')], snapshot, env)
    report['checks'].append(check)
    save()
    if check['exitCode'] != 0:
        raise RuntimeError('Option/Result executable harness failed')
    check = invoke('native-optional-patterns-and-boundaries', [tool,'run',str(snapshot / 'test/optionsyntax.go')], snapshot, env)
    report['checks'].append(check)
    save()
    if check['exitCode'] != 0:
        raise RuntimeError('native optional executable harness failed')
    smoke = (out / 'bootstrap.legacy-smoke.source.txt').read_text()
    smoke_source = out / 'bootstrap.legacy-smoke.source.txt'
    smoke_source.write_text(smoke)
    smoke_path = temp / 'legacy-smoke.go'
    smoke_path.write_text(smoke)
    report['legacySmokeSource'] = str(smoke_source)
    for name, command in [('legacy-go-smoke', [tool,'run',str(smoke_path)]), ('legacy-go-smoke-baseline', [baseline,'run',str(smoke_path)])]:
        check = invoke(name, command, snapshot, env)
        report['checks'].append(check)
        save()
        if check['exitCode'] != 0:
            raise RuntimeError(name+' failed')
    matched = (out / 'bootstrap.legacy-go-smoke.log').read_bytes() == (out / 'bootstrap.legacy-go-smoke-baseline.log').read_bytes()
    report['legacySmokeBaselineCompared'] = matched
    if not matched:
        raise RuntimeError('legacy smoke output differs from baseline')
    report['status'] = 'pass'
except Exception as error:
    report['status'] = 'fail'
    report['failure'] = repr(error)
    print('Failure: '+str(error), flush=True)
finally:
    shutil.rmtree(temp)
    report.update(temporarySnapshotRemoved=True, totalSeconds=round(time.monotonic()-start,3), finishedAt=now())
    save()
    print('Final: '+report['status']+', snapshot removed, '+str(report['totalSeconds'])+'s', flush=True)
    if report['status'] != 'pass':
        raise SystemExit(1)
