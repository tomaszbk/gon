#!/usr/bin/env python3
"""End-to-end checks of the gon tooling commands through the public launcher.

Runs a legacy/modern program pair with Gon and the legacy program with an
unmodified Go toolchain, analyzes and renames both with gon query/check/refactor,
checks that Go commands keep their behavior, and that unsaved editor buffers
never reach command-line results.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from test import Client, position

ROOT = Path(__file__).resolve().parents[2]
SUFFIX = ".exe" if os.name == "nt" else ""
GON = ROOT / "gon" / "bin" / ("gon" + SUFFIX)

STORE_MODERN = '''package store

import (
	"errors"
	"fmt"
)

type Memory struct{ Values map[string]int }

func (m *Memory) Read(key string) (int, error) {
	v, ok := m.Values[key]
	if !ok {
		return 0, errors.New("missing " + key)
	}
	return v, nil
}

// Sum adds two stored values.
func Sum(m *Memory, a, b string) (int, error) {
	x := m.Read(a)!
	y := m.Read(b) or problem {
		return 0, fmt.Errorf("second value: %w", problem)
	}
	return x + y, nil
}
'''
STORE_LEGACY = STORE_MODERN.replace('''	x := m.Read(a)!
	y := m.Read(b) or problem {
		return 0, fmt.Errorf("second value: %w", problem)
	}''', '''	x, err := m.Read(a)
	if err != nil {
		return 0, err
	}
	y, problem := m.Read(b)
	if problem != nil {
		return 0, fmt.Errorf("second value: %w", problem)
	}''')
MAIN = '''package main

import (
	"fmt"

	"example.com/pair/store"
)

func main() {
	m := &store.Memory{Values: map[string]int{"a": 1, "b": 2}}
	fmt.Println(store.Sum(m, "a", "b"))
	fmt.Println(store.Sum(m, "a", "c"))
	fmt.Println(store.Sum(m, "c", "a"))
}
'''
TEST = '''package store

import "testing"

func TestSum(t *testing.T) {
	if v, err := Sum(&Memory{Values: map[string]int{"a": 1}}, "a", "a"); v != 2 || err != nil {
		t.Fatal(v, err)
	}
}
'''
EXPECTED = "3 <nil>\n0 second value: missing c\n0 missing c\n"


def run(*args, cwd, env=None, check=True):
    proc = subprocess.run([str(a) for a in args], cwd=cwd, env=env, text=True, capture_output=True)
    if check and proc.returncode != 0:
        raise AssertionError(f"{args}: exit {proc.returncode}\n{proc.stdout}\n{proc.stderr}")
    return proc


def gon_json(*args, cwd, env, code=0):
    proc = run(GON, *args, "--json", cwd=cwd, env=env, check=False)
    assert proc.returncode == code, (args, proc.returncode, proc.stdout, proc.stderr)
    return json.loads(proc.stdout)


def write_module(folder, store):
    (folder / "store").mkdir(parents=True)
    (folder / "go.mod").write_text("module example.com/pair\n\ngo 1.26\n")
    (folder / "main.go").write_text(MAIN)
    (folder / "store" / "store.go").write_text(store)
    (folder / "store" / "store_test.go").write_text(TEST)


def main():
    baseline = os.environ.get("GON_BASELINE_GO")
    if not baseline:
        raise SystemExit("Set GON_BASELINE_GO to an unmodified Go executable")
    with tempfile.TemporaryDirectory(prefix="gon-cli-") as temp:
        temp = Path(temp).resolve()
        env = dict(os.environ)
        legacy, modern = temp / "legacy", temp / "modern"
        write_module(legacy, STORE_LEGACY)
        write_module(modern, STORE_MODERN)
        # Go commands behave as before.
        assert run(baseline, "run", ".", cwd=legacy).stdout == EXPECTED
        for folder in (legacy, modern):
            assert run(GON, "run", ".", cwd=folder, env=env).stdout == EXPECTED
            run(GON, "build", "./...", cwd=folder, env=env)
            run(GON, "vet", "./...", cwd=folder, env=env)
            run(GON, "test", "./...", cwd=folder, env=env)
            assert run(GON, "fmt", "./...", cwd=folder, env=env).stdout == ""
        assert "go version go1." in run(GON, "version", cwd=modern, env=env).stdout
        assert "usage: go build" in run(GON, "help", "build", cwd=modern, env=env).stdout

        caps = gon_json("capabilities", cwd=modern, env=env)
        assert caps["schemaVersion"] == 1 and caps["operation"] == "capabilities", caps
        assert caps["capabilities"]["check"] and caps["capabilities"]["rename"], caps
        assert caps["toolchain"]["root"] == str(ROOT), caps
        assert caps["toolchain"]["languageServer"] == str(ROOT / "gon" / "bin" / ("gonpls" + SUFFIX)), caps

        # Both spellings check clean and have the same references.
        counts = {}
        for folder in (legacy, modern):
            result = gon_json("check", "./...", cwd=folder, env=env)
            assert result["ok"] and result["summary"]["errors"] == 0, result
            refs = gon_json("query", "refs", "store.Sum", "store.Memory.Read", cwd=folder, env=env)
            counts[folder.name] = [r["total"] for r in refs["results"]]
        assert counts["legacy"] == counts["modern"] == [5, 3], counts  # includes the test

        # The modern handler binding and constructs are understood.
        binding = gon_json("query", "refs", "store/store.go:" + str(STORE_MODERN.count("\n", 0, STORE_MODERN.index("problem {")) + 1) + ":20",
                           cwd=modern, env=env)
        assert binding["results"][0]["total"] == 2, binding
        types = gon_json("query", "type", "store/store.go:20:16", cwd=modern, env=env)
        assert types["results"][0]["type"]["construct"] == "error-propagation", types

        # Invalid new syntax is a language error with an explainable code.
        bad = modern / "bad"
        bad.mkdir()
        (bad / "bad.go").write_text("package bad\n\nfunc f() int { return 1 }\n\nfunc g() error {\n\tf()!\n\treturn nil\n}\n")
        result = gon_json("check", "./bad", cwd=modern, env=env, code=1)
        assert [d["code"] for d in result["diagnostics"]] == ["InvalidErrorHandling"], result
        assert gon_json("explain", "InvalidErrorHandling", cwd=modern, env=env)["explanations"][0]["cases"]
        (bad / "bad.go").unlink()
        bad.rmdir()

        # Renames produce the same program in both spellings; the legacy
        # result still runs with the unmodified toolchain.
        for folder in (legacy, modern):
            plan = gon_json("refactor", "rename", "store.Sum", "Add", "--dry-run", cwd=folder, env=env)
            assert plan["status"] == "planned" and plan["summary"] == {"files": 3, "edits": 6}, plan  # includes the doc comment
            applied = gon_json("refactor", "rename", "store.Sum", "Add", cwd=folder, env=env)
            assert applied["status"] == "applied", applied
            assert run(GON, "run", ".", cwd=folder, env=env).stdout == EXPECTED
            assert gon_json("query", "refs", "store.Add", cwd=folder, env=env)["results"][0]["total"] == 5
        assert run(baseline, "run", ".", cwd=legacy).stdout == EXPECTED
        assert run(baseline, "vet", "./...", cwd=legacy).returncode == 0

        # An editor's unsaved buffer is invisible to the command line
        # until it is saved.
        main_go = modern / "main.go"
        renamed = MAIN.replace("store.Sum", "store.Add")
        assert main_go.read_text() == renamed
        last = '\tfmt.Println(store.Add(m, "c", "a"))\n'
        edited = renamed.replace(last, last + '\tfmt.Println(store.Add(m, "b", "b"))\n')
        with tempfile.TemporaryFile(mode="w+") as log:
            client = Client(modern, log)
            try:
                client.send("textDocument/didOpen", {"textDocument": {
                    "uri": main_go.as_uri(), "languageId": "gon", "version": 1, "text": edited}})
                uses = client.request("textDocument/references", {
                    "textDocument": {"uri": main_go.as_uri()},
                    "position": position(edited, 'Add(m, "a", "b")'),
                    "context": {"includeDeclaration": True}})
                assert len(uses) == 6, uses  # the editor sees its buffer
                assert gon_json("query", "refs", "store.Add", cwd=modern, env=env)["results"][0]["total"] == 5
                main_go.write_text(edited)
                after = gon_json("query", "refs", "store.Add", cwd=modern, env=env)
                assert after["results"][0]["total"] == 6, after
            finally:
                client.close()

        # The skill is copied into a project, never installed globally.
        install = ROOT / "misc" / "gon" / "install.py"
        project = temp / "project"
        project.mkdir()
        run(sys.executable, install, "--project-skill", project, cwd=temp)
        run(sys.executable, install, "--project-skill", project, cwd=temp)
        copied = project / ".agents" / "skills" / "gon" / "SKILL.md"
        assert copied.read_text() == (ROOT / ".agents" / "skills" / "gon" / "SKILL.md").read_text()
        copied.write_text("local changes")
        assert run(sys.executable, install, "--project-skill", project, cwd=temp, check=False).returncode != 0
        assert copied.read_text() == "local changes"
    print("PASS: Go commands, capabilities, legacy/modern check, queries, "
          "rename and baseline execution, invalid syntax, editor buffer isolation, project skill")


if __name__ == "__main__":
    main()
