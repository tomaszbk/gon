#!/usr/bin/env python3
"""Executable and real stdio-LSP regression tests for Gon public commands."""
import json
import os
from pathlib import Path
import queue
import sys
import subprocess
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[2]
BIN = ROOT / "gon" / "bin"
SUFFIX = ".exe" if os.name == "nt" else ""
GON, LSP = (BIN / (n + SUFFIX) for n in ("gon", "gonpls"))

MODERN = '''package main

import (
    "errors"
    "fmt"
)

func read(fail bool) (int, error) {
    if fail { return 7, errors.New("failure") }
    return 21, nil
}

func twice(fail bool) (int, error) {
    value := read(fail) or problem {
        return 0, fmt.Errorf("wrapped: %w", problem)
    }
    next := read(false)!
    return value + next, nil
}

func main() {
    a, e := twice(false)
    b, f := twice(true)
    fmt.Println(a, e, b, f)
}
'''
LEGACY = MODERN.replace('value := read(fail) or problem {', 'value, problem := read(fail)\n    if problem != nil {').replace(
    'next := read(false)!', 'next, err := read(false)\n    if err != nil { return 0, err }')

CONDITIONAL_MODERN = '''package main
import "fmt"
var calls, conditions int
func touch(v int) int { calls++; return v }
func condition(flag bool) bool { conditions++; return flag }
func choose(flag bool) int {
    return if condition(flag) { touch(1) } else { touch(2) }
}
func complete(flag bool, pickInt int, pickString string, pickBool bool) int {
    return if flag { pickInt } else { pickInt }
}
func inferred(flag bool, pickInt int, pickString string) {
    value := if flag { pickInt } else { pickInt }
    fmt.Println(value)
}
func main() {
    a, b := choose(true), choose(false)
    fmt.Println(a, b, calls, conditions)
}
'''
CONDITIONAL_LEGACY = CONDITIONAL_MODERN.replace(
    'return if condition(flag) { touch(1) } else { touch(2) }',
    'if condition(flag) { return touch(1) }; return touch(2)').replace(
    'return if flag { pickInt } else { pickInt }',
    'if flag { return pickInt }; return pickInt').replace(
    'value := if flag { pickInt } else { pickInt }',
    'var value int; if flag { value = pickInt } else { value = pickInt }')


def command(*args, cwd=None, env=None):
    return subprocess.run([str(a) for a in args], cwd=cwd, env=env,
                          check=True, text=True, capture_output=True).stdout


class Client:
    def __init__(self, folder, log, options=None, snippet_support=False):
        self.process = subprocess.Popen([str(LSP), "serve"], cwd=folder,
                                        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=log)
        self.messages = queue.Queue()
        self.notifications = []
        self.serial = 0
        threading.Thread(target=self.read, daemon=True).start()
        self.result = self.request("initialize", {
            "processId": os.getpid(), "rootUri": folder.as_uri(),
            "workspaceFolders": [{"uri": folder.as_uri(), "name": "gon-test"}],
            "capabilities": {"textDocument": {
                "publishDiagnostics": {"versionSupport": True},
                "completion": {"completionItem": {"snippetSupport": snippet_support}},
                "semanticTokens": {"requests": {"full": True}, "formats": ["relative"],
                    "tokenTypes": ["namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator"],
                    "tokenModifiers": ["declaration", "definition", "readonly", "static", "deprecated", "abstract", "async", "modification", "documentation", "defaultLibrary"]}}},
            "initializationOptions": dict({"semanticTokens": True, "staticcheck": True, "analyses": {"unusedwrite": True}, "diagnosticsDelay": "10ms"}, **(options or {})),
        })
        self.send("initialized", {})

    def read(self):
        try:
            while True:
                headers = {}
                while True:
                    line = self.process.stdout.readline()
                    if not line:
                        raise EOFError("gonpls stdout closed")
                    if line == b"\r\n":
                        break
                    key, value = line.decode().split(":", 1)
                    headers[key.lower()] = value.strip()
                self.messages.put(json.loads(self.process.stdout.read(int(headers["content-length"]))))
        except Exception as error:
            self.messages.put(error)

    def send(self, method, params, serial=None):
        msg = {"jsonrpc": "2.0", "method": method, "params": params}
        if serial is not None:
            msg["id"] = serial
        self.write(msg)

    def write(self, msg):
        body = json.dumps(msg).encode()
        self.process.stdin.write(f"Content-Length: {len(body)}\r\n\r\n".encode() + body)
        self.process.stdin.flush()

    def receive(self, timeout):
        msg = self.messages.get(timeout=timeout)
        if isinstance(msg, Exception):
            raise msg
        if "method" in msg and "id" in msg:
            # Acknowledge optional server requests; configuration is supplied
            # at initialization, so workspace/configuration isn't advertised.
            self.write({"jsonrpc": "2.0", "id": msg["id"], "result": None})
        elif "method" in msg:
            self.notifications.append(msg)
        return msg

    def request(self, method, params):
        self.serial += 1
        serial = self.serial
        self.send(method, params, serial)
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            msg = self.receive(max(0.01, deadline - time.monotonic()))
            if msg.get("id") == serial and "method" not in msg:
                assert "error" not in msg, (method, msg)
                return msg.get("result")
        raise TimeoutError(method)

    def diagnostics(self, uri, version, predicate):
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            for msg in self.notifications:
                p = msg.get("params", {})
                if (msg.get("method") == "textDocument/publishDiagnostics"
                    and p.get("uri") == uri and p.get("version") == version
                    and predicate(p["diagnostics"])):
                    return p["diagnostics"]
            try:
                self.receive(max(0.01, deadline - time.monotonic()))
            except queue.Empty:
                break
        raise TimeoutError((uri, version, self.notifications))

    def close(self):
        try:
            if self.process.poll() is not None:
                return
            self.request("shutdown", None)
            self.send("exit", None)
            self.process.wait(timeout=10)
            assert self.process.returncode == 0
        finally:
            if self.process.poll() is None:
                self.process.kill()
                self.process.wait()


def position(source, needle, offset=0):
    index = source.index(needle) + offset
    before = source[:index]
    return {"line": before.count("\n"), "character": len(before.rsplit("\n", 1)[-1])}


def apply_edits(source, edits):
    lines = source.splitlines(keepends=True)
    def offset(pos):
        return sum(map(len, lines[:pos["line"]])) + pos["character"]
    for edit in sorted(edits, key=lambda e: offset(e["range"]["start"]), reverse=True):
        start, end = (offset(edit["range"][p]) for p in ("start", "end"))
        source = source[:start] + edit["newText"] + source[end:]
    return source


def conditional_editor(baseline):
    """Real LSP requests, incomplete branches, and an executable lazy-flow pair."""
    with tempfile.TemporaryDirectory(prefix="gon-conditional-editor-") as temp:
        folder = Path(temp).resolve()
        (folder / "go.mod").write_text("module example.com/conditional\n\ngo 1.26\n")
        file = folder / "main.go"
        file.write_text(CONDITIONAL_LEGACY)
        expected = "1 2 2 2\n"
        assert command(baseline, "run", file, cwd=folder) == expected
        assert command(GON, "run", file, cwd=folder) == expected
        file.write_text(CONDITIONAL_MODERN)
        assert command(GON, "run", file, cwd=folder) == expected
        uri = file.as_uri()
        doc = {"textDocument": {"uri": uri}}
        with (folder / "lsp.log").open("w+") as log:
            client = Client(folder, log)
            try:
                client.send("textDocument/didOpen", {"textDocument": {
                    "uri": uri, "languageId": "go", "version": 1, "text": CONDITIONAL_MODERN}})
                diagnostics = client.diagnostics(uri, 1, lambda d: True)
                assert not any(d.get("severity") == 1 for d in diagnostics), diagnostics
                tokens = client.request("textDocument/semanticTokens/full", doc)
                legend = client.result["capabilities"]["semanticTokensProvider"]["legend"]["tokenTypes"]
                classified = {}
                line = column = 0
                for delta, char, length, kind, _ in zip(*[iter(tokens["data"])] * 5):
                    line += delta
                    column = char if delta else column + char
                    classified[(line, column, length)] = legend[kind]
                for needle, size in [("if condition", 2), ("else { touch", 4)]:
                    pos = position(CONDITIONAL_MODERN, needle)
                    assert classified[(pos["line"], pos["character"], size)] == "keyword", classified
                hover = client.request("textDocument/hover", dict(doc, position=position(CONDITIONAL_MODERN, "pickInt }")))
                assert "int" in json.dumps(hover), hover
                definitions = client.request("textDocument/definition", dict(doc, position=position(CONDITIONAL_MODERN, "pickInt }")))
                assert definitions[0]["range"]["start"] == position(CONDITIONAL_MODERN, "pickInt int"), definitions
                for needle in ("touch(1)", "touch(2)"):
                    actions = client.request("textDocument/codeAction", dict(doc,
                        range={"start": position(CONDITIONAL_MODERN, needle), "end": position(CONDITIONAL_MODERN, needle, len(needle))},
                        context={"diagnostics": [], "only": ["refactor.extract.variable", "refactor.extract.variable-all"]}))
                    assert not actions, actions
                # Partial identifiers in both branches use their value context;
                # a partial condition must prefer a boolean, not that value type.
                cases = [
                    ("return if flag { pickInt } else { pickInt }", "return if flag { pick } else { pickInt }", "pick }", "pickInt", "pickString"),
                    ("return if flag { pickInt } else { pickInt }", "return if flag { pickInt } else { pick }", "pick }", "pickInt", "pickString"),
                    ("return if flag { pickInt } else { pickInt }", "return if pick { pickInt } else { pickInt }", "pick {", "pickBool", "pickInt"),
                    ("value := if flag { pickInt } else { pickInt }", "value := if flag { pick } else { pickInt }", "pick }", "pickInt", "pickString"),
                ]
                for version, (old, new, needle, preferred, other) in enumerate(cases, 2):
                    source = CONDITIONAL_MODERN.replace(old, new)
                    client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": version}, contentChanges=[{"text": source}]))
                    completion = client.request("textDocument/completion", dict(doc, position=position(source, needle, len("pick"))))
                    labels = [item["label"] for item in completion["items"]]
                    assert preferred in labels and other in labels, labels
                    assert labels.index(preferred) < labels.index(other), labels
                    # Incomplete branch identifiers must not trigger semtok's
                    # unimplemented-node fallback either.
                    assert client.request("textDocument/semanticTokens/full", doc)["data"]
            finally:
                client.close()
            log.seek(0)
            logs = log.read()
            assert "panic" not in logs.lower() and "failed to implement" not in logs, logs



FEATURES_MODERN = '''package main
import ("fmt"; "strconv")
type Item struct { Value int; Next *Item }
func defaultValue() int { return -1 }
func complete(pickInt int, pickString string) {
    var f func(int) int = (n) => pickInt + n
    _ = f
}
func main() {
    var fn func(int) int = (x) => x + 1
    var p *Item
    a := p?.Value ?? defaultValue()
    p = &Item{Value: 7}
    b := p?.Value ?? defaultValue()
    var absent func(int) int
    fallback := absent ?? (x) => x + 2
    c := fallback?(3) ?? 0
    entries := map[int]*Item{}
    entries[0] ??= p
    var parseFn func(string) (int, error) = (s) => {
        v := strconv.Atoi(s)!
        return v, nil
    }
    n, err := parseFn("bad")
    fmt.Println(fn(3), a, b, c, entries[0].Value, n, err != nil)
}
'''
FEATURES_LEGACY = FEATURES_MODERN.replace(
    '(n) => pickInt + n', 'func(n int) int { return pickInt + n }').replace(
    '(x) => x + 1', 'func(x int) int { return x + 1 }').replace(
    'a := p?.Value ?? defaultValue()',
    'var a int; if p != nil { a = p.Value } else { a = defaultValue() }').replace(
    'b := p?.Value ?? defaultValue()',
    'var b int; if p != nil { b = p.Value } else { b = defaultValue() }').replace(
    'fallback := absent ?? (x) => x + 2',
    'fallback := absent; if fallback == nil { fallback = func(x int) int { return x + 2 } }').replace(
    'c := fallback?(3) ?? 0', 'var c int; if fallback != nil { c = fallback(3) }').replace(
    'entries[0] ??= p', 'if entries[0] == nil { entries[0] = p }').replace(
    '(s) => {', 'func(s string) (int, error) {').replace(
    'v := strconv.Atoi(s)!', 'v, err := strconv.Atoi(s); if err != nil { return 0, err }')


def features_editor(baseline):
    """Lambda/nil-safety execution pairs and real authoring/navigation requests."""
    with tempfile.TemporaryDirectory(prefix="gon-features-editor-") as temp:
        folder = Path(temp).resolve()
        (folder / "go.mod").write_text("module example.com/features\n\ngo 1.26\n")
        file = folder / "main.go"
        expected = "4 -1 7 5 7 0 true\n"
        file.write_text(FEATURES_LEGACY)
        assert command(baseline, "run", file, cwd=folder) == expected
        assert command(GON, "run", file, cwd=folder) == expected
        file.write_text(FEATURES_MODERN)
        assert command(GON, "run", file, cwd=folder) == expected
        uri = file.as_uri()
        doc = {"textDocument": {"uri": uri}}
        with (folder / "lsp.log").open("w+") as log:
            client = Client(folder, log, {"hints": {"assignVariableTypes": True}})
            try:
                client.send("textDocument/didOpen", {"textDocument": {
                    "uri": uri, "languageId": "go", "version": 1, "text": FEATURES_MODERN}})
                diagnostics = client.diagnostics(uri, 1, lambda d: True)
                assert not any(d.get("severity") == 1 for d in diagnostics), diagnostics
                tokens = client.request("textDocument/semanticTokens/full", doc)
                legend = client.result["capabilities"]["semanticTokensProvider"]["legend"]["tokenTypes"]
                classified = {}
                line = column = 0
                for delta, char, length, kind, _ in zip(*[iter(tokens["data"])] * 5):
                    line += delta
                    column = char if delta else column + char
                    classified[(line, column, length)] = legend[kind]
                for needle, size in [("=> x + 1", 2), ("?.Value", 2), ("?? defaultValue", 2), ("?(3)", 2), ("??= p", 3)]:
                    pos = position(FEATURES_MODERN, needle)
                    assert classified[(pos["line"], pos["character"], size)] == "operator", classified
                param = position(FEATURES_MODERN, "x) => x + 1")
                assert classified[(param["line"], param["character"], 1)] == "parameter", classified
                for needle, want in [("=> x + 1", "func"), ("pickInt + n", "int"), ("return v, nil", "returns (int, error)")]:
                    hover = client.request("textDocument/hover", dict(doc, position=position(FEATURES_MODERN, needle)))
                    assert want in json.dumps(hover), (needle, hover)
                definitions = client.request("textDocument/definition", dict(doc, position=position(FEATURES_MODERN, "x + 1")))
                assert definitions[0]["range"]["start"] == param, definitions
                definitions = client.request("textDocument/definition", dict(doc, position=position(FEATURES_MODERN, "return v, nil")))
                assert definitions[0]["range"]["start"] == position(FEATURES_MODERN, "=> {"), definitions
                signature = client.request("textDocument/signatureHelp", dict(doc, position=position(FEATURES_MODERN, "?(3)", 2)))
                assert "int" in json.dumps(signature), signature
                hints = client.request("textDocument/inlayHint", dict(doc, range={"start": {"line": 0, "character": 0}, "end": {"line": len(FEATURES_MODERN.splitlines())-1, "character": 1}}))
                assert any(h["position"] == dict(param, character=param["character"]+1) and "int" in json.dumps(h["label"]) for h in hints), hints
                for needle in ("(x) => x + 1", "defaultValue()", "pickInt + n"):
                    actions = client.request("textDocument/codeAction", dict(doc,
                        range={"start": position(FEATURES_MODERN, needle), "end": position(FEATURES_MODERN, needle, len(needle))},
                        context={"diagnostics": [], "only": ["refactor.extract.variable", "refactor.extract.variable-all"]}))
                    assert not actions, (needle, actions)
                for version, (old, new, needle, preferred) in enumerate([
                    ("pickInt + n", "pick + n", "pick + n", "pickInt"),
                    ("b := p?.Value", "b := p?.Val", "b := p?.Val", "Value"),
                    ("b := p?.Value ?? defaultValue()", "b := p?.", "b := p?.", "Value"),
                ], 2):
                    source = FEATURES_MODERN.replace(old, new)
                    client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": version}, contentChanges=[{"text": source}]))
                    length = len("pick") if needle.startswith("pick") else len(needle)
                    completion = client.request("textDocument/completion", dict(doc, position=position(source, needle, length)))
                    labels = [item["label"] for item in completion["items"]]
                    assert preferred in labels, (preferred, labels)
                    if preferred == "pickInt":
                        assert labels.index("pickInt") < labels.index("pickString"), labels
                    assert client.request("textDocument/semanticTokens/full", doc)["data"]
                client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": 5}, contentChanges=[{"text": FEATURES_MODERN}]))
                # Expand typed lambdas only when the signature is nameable and
                # replacing them cannot change an enclosing generic inference.
                for ordinal, needle in enumerate(("=> x + 1", "=> {")):
                    version = 6 + 2*ordinal
                    actions = client.request("textDocument/codeAction", dict(doc,
                        range={"start": position(FEATURES_MODERN, needle), "end": position(FEATURES_MODERN, needle, 2)},
                        context={"diagnostics": [], "only": ["refactor.rewrite.lambda"]}))
                    assert len(actions) == 1 and actions[0]["title"] == "Convert lambda to function literal", actions
                    edits = [edit for change in actions[0]["edit"]["documentChanges"] for edit in change.get("edits", [])]
                    rewritten = apply_edits(FEATURES_MODERN, edits)
                    assert "func(" in rewritten, rewritten
                    file.write_text(rewritten)
                    assert command(GON, "run", file, cwd=folder) == expected
                    client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": version}, contentChanges=[{"text": rewritten}]))
                    literal = "func(x int) int" if ordinal == 0 else "func(s string) (int, error)"
                    inverse = client.request("textDocument/codeAction", dict(doc,
                        range={"start": position(rewritten, literal), "end": position(rewritten, literal, 4)},
                        context={"diagnostics": [], "only": ["refactor.rewrite.lambda"]}))
                    assert len(inverse) == 1 and inverse[0]["title"] == "Convert function literal to lambda", inverse
                    inverse_edits = [edit for change in inverse[0]["edit"]["documentChanges"] for edit in change.get("edits", [])]
                    roundtrip = apply_edits(rewritten, inverse_edits)
                    assert "=> {" in roundtrip, roundtrip
                    file.write_text(roundtrip)
                    assert command(GON, "run", file, cwd=folder) == expected
                    file.write_text(FEATURES_MODERN)
                    client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": version+1}, contentChanges=[{"text": FEATURES_MODERN}]))
                negative = FEATURES_MODERN + '''
func generic[T any](x T, f func(T) T) T { return f(x) }
func generic2[T, U any](x T, f func(T) U) U { return f(x) }
type UintFn func(uint) uint
func rejectRewrite() {
    _ = generic(1, (x) => x + 1)
    _ = generic2[int](1, (x) => x)
    uint := 1
    var f UintFn = (v) => v
    _, _ = uint, f
    _ = generic(1, func(genericParam int) int { return genericParam })
}
var commented func(int) int = (x) => /*keep*/ x
var inferred = func(inferredParam int) int { return inferredParam }
var named func(int) int = func(namedParam int) (result int) { return namedParam }
var boxed any = func(boxedParam int) int { return boxedParam }
var parenthesized func(int) int = (func(parenParam int) int { return parenParam })
'''
                client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": 10}, contentChanges=[{"text": negative}]))
                for needle in ("=> x + 1)\n", "=> x)\n", "=> /*keep*/ x", "=> v\n", "func(genericParam", "func(inferredParam", "func(namedParam", "func(boxedParam", "func(parenParam"):
                    actions = client.request("textDocument/codeAction", dict(doc,
                        range={"start": position(negative, needle), "end": position(negative, needle, 2)},
                        context={"diagnostics": [], "only": ["refactor.rewrite.lambda"]}))
                    assert not actions, (needle, actions)
                client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": 11}, contentChanges=[{"text": FEATURES_MODERN}]))
                edits = client.request("textDocument/formatting", dict(doc, options={"tabSize": 4, "insertSpaces": False}))
                formatted = apply_edits(FEATURES_MODERN, edits)
                assert "=>" in formatted and "?.Value" in formatted and " ??= " in formatted
                file.write_text(formatted)
                assert command(GON, "run", file, cwd=folder) == expected
            finally:
                client.close()
            log.seek(0)
            logs = log.read()
            assert "panic" not in logs.lower() and "failed to implement" not in logs, logs

def main():
    baseline = os.environ.get("GON_BASELINE_GO") or os.environ.get("GO_ERROR_HANDLING_BASELINE")
    if not baseline:
        raise SystemExit("Set GON_BASELINE_GO to an unmodified Go executable")
    if sys.argv[1:] == ["--features-only"]:
        features_editor(baseline)
        print("PASS: lambda/null-safety executable pair, LSP tokens, hover, definitions, signature help, inferred parameter hints, completion, extraction safety and formatting")
        return
    if sys.argv[1:] == ["--conditional-only"]:
        conditional_editor(baseline)
        print("PASS: conditional baseline/modern execution, LSP tokens, hover, definition, incomplete completion and extraction safety")
        return
    before = command(baseline, "version")
    with tempfile.TemporaryDirectory(prefix="gon-tools-test-") as temp:
        folder = Path(temp).resolve()
        # Installer is idempotent, resolves symlinks, and refuses collisions.
        public = folder / "public"
        install = ROOT / "misc" / "gon" / "install.py"
        command(sys.executable, install, "--no-modify-path", "--bin-dir", public)
        command(sys.executable, install, "--no-modify-path", "--bin-dir", public)
        assert command(public / ("gon" + SUFFIX), "env", "GOROOT").strip() == str(ROOT)
        other = folder / "occupied"
        other.mkdir()
        (other / ("gon" + SUFFIX)).write_text("keep me")
        refused = subprocess.run([sys.executable, str(install), "--bin-dir", str(other)], capture_output=True)
        assert refused.returncode != 0
        assert (other / ("gon" + SUFFIX)).read_text() == "keep me"
        assert not (other / ("gonpls" + SUFFIX)).exists()
        (folder / "go.mod").write_text("module example.com/gon-test\n\ngo 1.26\n")
        file = folder / "main.go"
        file.write_text(LEGACY)
        expected = "42 <nil> 0 wrapped: failure\n"
        assert command(baseline, "run", file, cwd=folder) == expected
        assert command(GON, "run", file, cwd=folder) == expected
        file.write_text(MODERN)
        assert command(GON, "run", file, cwd=folder) == expected
        assert command(GON, "test", "./...", cwd=folder)
        command(GON, "vet", "./...", cwd=folder)
        # An inherited upstream GOROOT/toolchain must not override Gon.
        env = dict(os.environ, GOROOT=command(baseline, "env", "GOROOT").strip(), GOTOOLCHAIN="auto")
        assert command(GON, "run", file, cwd=folder, env=env) == expected
        assert command(GON, "env", "GOTOOLCHAIN", cwd=folder, env=env).strip() == "local"
        failed = subprocess.run([str(GON), "build", "./does-not-exist"], cwd=folder, capture_output=True)
        assert failed.returncode != 0
        uri = file.as_uri()
        doc = {"textDocument": {"uri": uri}}
        with (folder / "lsp.log").open("w+") as log:
            client = Client(folder, log)
            try:
                assert client.result["serverInfo"]["name"] == "gonpls", client.result
                assert all(c.startswith("gonpls.") for c in client.result["capabilities"]["executeCommandProvider"]["commands"])
                client.send("textDocument/didOpen", {"textDocument": {
                    "uri": uri, "languageId": "go", "version": 1, "text": MODERN}})
                diagnostics = client.diagnostics(uri, 1, lambda d: True)
                assert not [d for d in diagnostics if d.get("severity") == 1], diagnostics
                hover = client.request("textDocument/hover", dict(doc, position=position(MODERN, "problem)")))
                assert "error" in json.dumps(hover), hover
                imports = client.request("workspace/executeCommand", {"command": "gonpls.list_imports", "arguments": [{"URI": uri}]})
                assert "fmt" in json.dumps(imports), imports
                definitions = client.request("textDocument/definition", dict(doc, position=position(MODERN, "problem)")))
                assert definitions and definitions[0]["range"]["start"] == position(MODERN, "problem {"), definitions
                rename = client.request("textDocument/rename", dict(doc, position=position(MODERN, "problem)"), newName="failure"))
                assert json.dumps(rename).count('"newText": "failure"') == 2, rename
                completion = client.request("textDocument/completion", dict(doc, position=position(MODERN, "problem)", 3)))
                assert any(item["label"] == "problem" for item in completion["items"]), completion
                edits = client.request("textDocument/formatting", dict(doc, options={"tabSize": 4, "insertSpaces": False}))
                assert edits, "expected formatting edits"
                formatted = apply_edits(MODERN, edits)
                assert "or problem {" in formatted and "read(false)!" in formatted
                file.write_text(formatted)
                assert command(GON, "run", file, cwd=folder) == expected
                file.write_text(MODERN)
                tokens = client.request("textDocument/semanticTokens/full", doc)
                assert tokens and tokens["data"], tokens
                legend = client.result["capabilities"]["semanticTokensProvider"]["legend"]["tokenTypes"]
                classified = {}
                line = column = 0
                for delta, char, length, kind, _ in zip(*[iter(tokens["data"])] * 5):
                    line += delta
                    column = char if delta else column + char
                    classified[(line, column, length)] = legend[kind]
                for needle, size, kind in [("or problem", 2, "keyword"), ("!", 1, "operator")]:
                    pos = position(MODERN, needle)
                    assert classified[(pos["line"], pos["character"], size)] == kind, classified
                # Unsaved real errors must still be diagnosed, then cleared.
                broken = MODERN.replace("return value + next, nil", 'return "wrong" + next, nil')
                client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": 2}, contentChanges=[{"text": broken}]))
                errors = client.diagnostics(uri, 2, lambda ds: any(d.get("severity") == 1 for d in ds))
                assert any("mismatch" in d["message"] or "invalid operation" in d["message"] for d in errors), errors
                client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": 3}, contentChanges=[{"text": MODERN}]))
                client.diagnostics(uri, 3, lambda ds: not any(d.get("severity") == 1 for d in ds))
                unused = MODERN.replace('"errors"', '"errors"\n    "strings"')
                client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": 4}, contentChanges=[{"text": unused}]))
                actions = client.request("textDocument/codeAction", dict(doc, range={"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 0}}, context={"diagnostics": [], "only": ["source.organizeImports"]}))
                action = next(a for a in actions if a.get("kind") == "source.organizeImports")
                changes = action["edit"].get("documentChanges", [])
                import_edits = [e for change in changes for e in change.get("edits", [])]
                import_edits += action["edit"].get("changes", {}).get(uri, [])
                organized = apply_edits(unused, import_edits)
                assert '"strings"' not in organized, action
                file.write_text(organized)
                assert command(GON, "run", file, cwd=folder) == expected
                # The dedicated VS Code extension uses a distinct language ID
                # to coexist with official Go providers in the same window.
                client.send("textDocument/didClose", doc)
                client.send("textDocument/didOpen", {"textDocument": {
                    "uri": uri, "languageId": "gon", "version": 5, "text": MODERN}})
                client.diagnostics(uri, 5, lambda ds: not any(d.get("severity") == 1 for d in ds))
                hover = client.request("textDocument/hover", dict(doc, position=position(MODERN, "problem)")))
                assert "error" in json.dumps(hover), hover
                # Both independent flow builders must produce real diagnostics
                # on a package that uses Gon syntax, not silently skip analysis.
                analyzed = MODERN + """
func analysisValues() (int, string, error) { return 1, "value", nil }
func analysisProbe() error {
    x := read(false)!
    x = 7
    fmt.Println(x)
    s := struct{ value int }{}
    s.value = read(false)!
    a, b := analysisValues()!
    a, b = 2, "replaced"
    fmt.Println(a, b)
    return nil
}
"""
                client.send("textDocument/didChange", dict(doc, textDocument={"uri": uri, "version": 6}, contentChanges=[{"text": analyzed}]))
                findings = client.diagnostics(uri, 6, lambda ds: {"SA4006", "unusedwrite"}.issubset({str(d.get("source")) for d in ds}))
                assert not any(d.get("severity") == 1 for d in findings), findings
                messages = {d["message"] for d in findings if d.get("source") == "SA4006"}
                assert {"this value of a is never used", "this value of b is never used"}.issubset(messages), findings
                # Cancel real code actions over the wire. A race may complete
                # successfully, but a cancellation must never use error code 0.
                for _ in range(3):
                    client.serial += 1
                    request_id = client.serial
                    client.send("textDocument/codeAction", dict(doc,
                        range={"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 0}},
                        context={"diagnostics": [], "only": ["source.organizeImports"]}), request_id)
                    client.send("$/cancelRequest", {"id": request_id})
                    while True:
                        reply = client.receive(30)
                        if reply.get("id") == request_id and "method" not in reply:
                            if "error" in reply:
                                assert reply["error"]["code"] in (-32800, -32802), reply
                            break

            finally:
                try:
                    client.close()
                finally:
                    log.seek(0)
                    logs = log.read()
                    if client.process.returncode != 0 or "panic" in logs.lower():
                        print(logs, flush=True)
            log.seek(0)
            logs = log.read()
            assert "panic" not in logs.lower() and "failed to implement" not in logs, logs
    conditional_editor(baseline)
    features_editor(baseline)
    assert command(baseline, "version") == before
    print("PASS: executable legacy/modern pair, baseline, isolation, CLI errors, LSP diagnostics, hover, definition, rename, completion, formatting, semantic tokens, imports, unsaved edits, SSA/Staticcheck diagnostics")


if __name__ == "__main__":
    main()
