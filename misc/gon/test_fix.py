#!/usr/bin/env python3
"""Check Gon syntax suggestions, preview/application, LSP, and executable pairs."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

from test import Client

ROOT = Path(__file__).resolve().parents[2]
GON = ROOT / "gon/bin" / ("gon.exe" if os.name == "nt" else "gon")
ANALYZERS = {"gonerrors", "gonconditional", "gonnil", "gonlambda"}
EXPECTED = "PASS: errors, partial results, named returns, lazy branches, nil interfaces, closures\n"


def run(*args, cwd, check=True):
    env = dict(os.environ, GOTOOLCHAIN="local", GOWORK="off", GOFLAGS="")
    env.pop("GOROOT", None)
    env.pop("GOTOOLDIR", None)
    proc = subprocess.run([str(a) for a in args], cwd=cwd, env=env,
                          text=True, capture_output=True, timeout=120)
    if check and proc.returncode:
        raise AssertionError(f"{args}: exit {proc.returncode}\n{proc.stdout}\n{proc.stderr}")
    return proc


def test_error_context_suffixes(baseline):
    common = '''package main
import "fmt"
type failure string
func (f failure) Error() string { return string(f) }
var problem error = failure("failed")
type value struct { Inner int }
type wrapped struct { Inner error }
func (w wrapped) Error() string { return "wrapped: " + w.Inner.Error() }
func readValue(fail bool) (value, error) { if fail { return value{}, problem }; return value{41}, nil }
func readNumber(fail bool) (int, error) { if fail { return 0, problem }; return 41, nil }
func main() {
    for _, f := range []func(bool) (int, error){selector, binary} {
        n, err := f(false)
        if n != 42 || err != nil { panic("success suffix") }
        n, err = f(true)
        if n != 0 || err.(wrapped).Inner != problem { panic("error context suffix") }
    }
    fmt.Println("PASS context suffixes")
}
'''
    legacy = '''
func selector(fail bool) (int, error) {
    v, err := readValue(fail)
    if err != nil { return 0, wrapped{err} }
    return v.Inner + 1, nil
}
func binary(fail bool) (int, error) {
    v, err := readNumber(fail)
    if err != nil { return 0, wrapped{err} }
    return v + 1, nil
}
'''
    modern = '''
func selector(fail bool) (int, error) {
    v := readValue(fail) or err { return 0, wrapped{err} }.Inner
    return v + 1, nil
}
func binary(fail bool) (int, error) {
    v := readNumber(fail) or err { return 0, wrapped{err} } + 1
    return v, nil
}
'''
    expected = "PASS context suffixes\n"
    with tempfile.TemporaryDirectory(prefix="gon-fix-context-") as temp:
        folder = Path(temp)
        path = folder / "main.go"
        (folder / "go.mod").write_text("module example.com/contextfix\n\ngo 1.27\n")
        path.write_text(common + legacy)
        assert run(baseline, "run", ".", cwd=folder).stdout == expected
        assert run(GON, "run", ".", cwd=folder).stdout == expected
        path.write_text(common + modern)
        assert run(GON, "run", ".", cwd=folder).stdout == expected
        run(GON, "fix", "-gonerrors", ".", cwd=folder)
        fixed = path.read_text()
        assert "=> wrapped{err}).Inner" in fixed, fixed
        assert "=> wrapped{err}) + 1" in fixed, fixed
        assert run(GON, "run", ".", cwd=folder).stdout == expected
        assert run(GON, "run", "-gcflags=all=-l", ".", cwd=folder).stdout == expected


def main():
    baseline = os.environ.get("GON_BASELINE_GO")
    if not baseline or not Path(baseline).is_absolute():
        raise SystemExit("Set GON_BASELINE_GO to an absolute unmodified Go 1.27+ executable")
    version = run(baseline, "version", cwd=ROOT).stdout.strip()
    match = re.search(r"\bgo1\.(\d+)\.(\d+)\b", version)
    assert match and int(match[1]) >= 27, version
    platform = run(GON, "env", "GOOS", "GOARCH", cwd=ROOT).stdout.split()
    test_error_context_suffixes(baseline)
    with tempfile.TemporaryDirectory(prefix="gon-fix-") as temp:
        folder = Path(temp).resolve()
        path = folder / "main.go"
        (folder / "go.mod").write_text("module example.com/gonfix\n\ngo 1.27\n")
        path.write_bytes((ROOT / "misc/gon/fixfixtures/legacy.go").read_bytes())
        run(GON, "fmt", ".", cwd=folder)
        legacy = path.read_text()
        assert run(baseline, "run", ".", cwd=folder).stdout == EXPECTED
        assert run(GON, "run", ".", cwd=folder).stdout == EXPECTED

        # Hints are optional and available with exact mechanical edits.
        checked = json.loads(run(GON, "check", "--severity=hint", "--json", ".", cwd=folder).stdout)
        hints = [d for d in checked["diagnostics"] if d.get("code") in ANALYZERS]
        assert {d["code"] for d in hints} == ANALYZERS, checked
        assert all(d["severity"] == "hint" and d.get("fixes") for d in hints), hints
        explained = json.loads(run(GON, "explain", *sorted(ANALYZERS), "--json", cwd=folder).stdout)
        assert {e["code"] for e in explained["explanations"]} == ANALYZERS, explained
        assert path.read_text() == legacy

        # The editor gets the same four diagnostics and actual quick fixes.
        with tempfile.TemporaryFile(mode="w+") as log:
            client = Client(folder, log, {"staticcheck": False})
            try:
                uri = path.as_uri()
                client.send("textDocument/didOpen", {"textDocument": {
                    "uri": uri, "languageId": "gon", "version": 1, "text": legacy}})
                diagnostics = client.diagnostics(uri, 1, lambda ds:
                    ANALYZERS.issubset({d.get("source") for d in ds}))
                for name in sorted(ANALYZERS):
                    diagnostic = next(d for d in diagnostics if d.get("source") == name)
                    actions = client.request("textDocument/codeAction", {
                        "textDocument": {"uri": uri}, "range": diagnostic["range"],
                        "context": {"diagnostics": [diagnostic], "only": ["quickfix"]}})
                    assert actions and any(a.get("edit") or a.get("command") for a in actions), (name, actions)
            finally:
                client.close()

        # Preview follows go fix's nonzero-on-diff convention and never writes.
        preview = run(GON, "fix", "-diff", ".", cwd=folder, check=False)
        assert preview.returncode == 1 and "read(fail)!" in preview.stdout, preview
        assert path.read_text() == legacy
        for name in ANALYZERS:
            assert name in run(GON, "tool", "fix", "help", name, cwd=folder).stdout

        run(GON, "fix", ".", cwd=folder)
        modern = path.read_text()
        for syntax in ("read(fail)!", "or err {", "flush(fail)!", "return if flag", "??=", "??", "?.Value", "?(touch", "(x) =>"):
            assert syntax in modern, (syntax, modern)
        # Unsafe removal of partial values or live error bindings is refused.
        for name in ("partial", "reused"):
            before = re.search(r"func " + name + r"\(.*?\n\}", legacy, re.S)[0]
            after = re.search(r"func " + name + r"\(.*?\n\}", modern, re.S)[0]
            assert before == after, (before, after)
        assert run(GON, "run", ".", cwd=folder).stdout == EXPECTED
        assert run(GON, "run", "-gcflags=all=-l", ".", cwd=folder).stdout == EXPECTED
        run(GON, "vet", ".", cwd=folder)
        # All fixes reach a stable state, including interacting analyzers.
        stable = run(GON, "fix", "-diff", ".", cwd=folder)
        assert not stable.stdout, stable.stdout
        assert path.read_text() == modern
        checked = json.loads(run(GON, "check", "--severity=hint", "--json", ".", cwd=folder).stdout)
        assert not [d for d in checked["diagnostics"] if d.get("code") in ANALYZERS], checked
    print(f"PASS: syntax fixes, preview, idempotence, CLI/LSP suggestions, executable legacy/modern pairs ({'/'.join(platform)}; {version})")


if __name__ == "__main__":
    main()
