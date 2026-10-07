#!/usr/bin/env python3
"""Executed optional-syntax pairs and real gonpls editor regressions."""
import json
import os
from pathlib import Path
import tempfile
import subprocess
from test import Client, GON, ROOT, command as run_command, position, apply_edits

def command(*args, **kwargs):
    try:
        return run_command(*args, **kwargs)
    except subprocess.CalledProcessError as error:
        raise AssertionError((error.cmd, error.stdout, error.stderr)) from error


BASELINE = os.environ.get('GON_BASELINE_GO')
if not BASELINE:
    raise SystemExit('Set GON_BASELINE_GO to the unmodified Go 1.27+ baseline')
ENV = {key: value for key, value in os.environ.items() if key not in ('GOROOT', 'GOTOOLDIR', 'GOFLAGS')}
ENV.update(GOENV='off', GOTOOLCHAIN='local')

LIB = '''package data
import "strconv"
// Number is a count passed through optional type syntax.
type Number int
// Parse returns the parsed port, absence, or a conversion failure.
func Parse(text string) (port int?, err error) {
    if text == "" { return nil, nil }
    value, problem := strconv.Atoi(text)
    if problem != nil { return nil, problem }
    return value, nil
}
// Present returns a present Number.
func Present(value Number) (present Number?) { return value }
'''
MAIN = '''package main
import ("fmt"; "example.com/simplified/data")
type Choice int
var declared data.Number? = 4
func ready(chosenInt int, chosenText string) (port int?) {
    _ = chosenText
    return chosenInt
}
func present(chosenInt int, chosenText string) int? {
    _ = chosenText
    return chosenInt
}
func absent() int? { return nil }
func extract(value data.Number?) data.Number { return switch value { case nil => 0; case number? => number } }
func main() {
    port := data.Parse(text: "23") or failure { panic(failure) }
    readyPort := ready(chosenInt: 7, chosenText: "unused")
    fmt.Println(port ?? 0, data.Present(value: 4) ?? 0, present(chosenInt: 5, chosenText: "unused") ?? 0, declared ?? 0, readyPort ?? 0, absent() ?? -1)
}
'''


def decoded_tokens(client, response):
    legend = client.result['capabilities']['semanticTokensProvider']['legend']['tokenTypes']
    line = char = 0
    result = {}
    for offset in range(0, len(response['data']), 5):
        delta_line, delta_char, length, kind, _ = response['data'][offset:offset+5]
        char = delta_char if delta_line else char + delta_char
        line += delta_line
        result[(line, char)] = (length, legend[kind])
    return result


def changes_of(edit):
    return {change['textDocument']['uri']: change['edits'] for change in edit['documentChanges']}


fixtures = ROOT / 'test' / 'optionsyntax.dir'
common, legacy, modern = (fixtures / name for name in ('common.go', 'legacy.go', 'modern.go'))
expected = command(BASELINE, 'run', common, legacy, env=ENV)
assert expected and command(GON, 'run', common, legacy, env=ENV) == expected
assert command(GON, 'run', common, modern, env=ENV) == expected
assert command(GON, 'run', '-gcflags=-l', common, modern, env=ENV) == expected

with tempfile.TemporaryDirectory(prefix='gon-simplification-editor-') as temporary:
    folder = Path(temporary).resolve()
    (folder / 'go.mod').write_text('module example.com/simplified\n\ngo 1.27\n')
    lib = folder / 'data' / 'data.go'
    lib.parent.mkdir()
    lib.write_text(LIB)
    main = folder / 'main.go'
    main.write_text(MAIN)
    assert command(GON, 'run', '.', cwd=folder, env=ENV) == '23 4 5 4 7 -1\n'
    uri, liburi = main.as_uri(), lib.as_uri()
    doc = {'textDocument': {'uri': uri}}
    with (folder / 'lsp.log').open('w+') as log:
        client = Client(folder, log, snippet_support=True)
        try:
            for fileuri, source in [(uri, MAIN), (liburi, LIB)]:
                client.send('textDocument/didOpen', {'textDocument': {'uri': fileuri, 'languageId': 'gon', 'version': 1, 'text': source}})
                diagnostics = client.diagnostics(fileuri, 1, lambda _: True)
                assert not any(d.get('severity') == 1 for d in diagnostics), diagnostics

            number = position(MAIN, 'data.Number?', len('data.'))
            definitions = client.request('textDocument/definition', dict(doc, position=number))
            assert definitions[0]['uri'] == liburi and definitions[0]['range']['start'] == position(LIB, 'Number int'), definitions
            hover = client.request('textDocument/hover', dict(doc, position=position(MAIN, 'data.Parse', len('data.'))))
            signature = hover['contents']['value']
            assert 'Parse(text string)' in signature and 'port int?' in signature and 'int?' in signature, hover
            definitions = client.request('textDocument/definition', dict(doc, position=position(MAIN, 'return chosenInt', len('return '))))
            assert definitions[0]['range']['start'] == position(MAIN, 'ready(chosenInt', len('ready(')), definitions
            refs = client.request('textDocument/references', {'textDocument': {'uri': liburi}, 'position': position(LIB, 'Number int'), 'context': {'includeDeclaration': True}})
            assert any(ref['uri'] == uri and ref['range']['start'] == number for ref in refs), refs
            renamed = client.request('textDocument/rename', {'textDocument': {'uri': liburi}, 'position': position(LIB, 'Number int'), 'newName': 'Count'})
            changes = changes_of(renamed)
            assert 'data.Count?' in apply_edits(MAIN, changes[uri]), changes
            assert 'present Count?' in apply_edits(LIB, changes[liburi]), changes
            lib.write_text(apply_edits(LIB, changes[liburi]))
            main.write_text(apply_edits(MAIN, changes[uri]))
            assert command(GON, 'run', '.', cwd=folder, env=ENV) == '23 4 5 4 7 -1\n'
            lib.write_text(LIB)
            main.write_text(MAIN)

            tokens = decoded_tokens(client, client.request('textDocument/semanticTokens/full', doc))
            for needle, offset, kind in [
                ('data.Number?', len('data.Number'), 'operator'),
                ('number? =>', len('number'), 'operator'),
                ('number? =>', 0, 'variable'),
            ]:
                point = position(MAIN, needle, offset)
                token = tokens.get((point['line'], point['character']))
                assert token and token[1] == kind, (needle, kind, token, tokens)
            edits = client.request('textDocument/formatting', dict(doc, options={'tabSize': 4, 'insertSpaces': False}))
            formatted = apply_edits(MAIN, edits or [])
            assert 'data.Number?' in formatted and 'port int?' in formatted and 'return chosenInt' in formatted and 'case number?' in formatted, formatted
            cases = [
                ('var declared data.Number? = 4', 'var declared Cho? = 4', 'var declared Cho', ['Choice'], ['chosenInt']),
            ]
            for version, (old, new, needle, names, absent) in enumerate(cases, 2):
                source = MAIN.replace(old, new)
                client.send('textDocument/didChange', {'textDocument': {'uri': uri, 'version': version}, 'contentChanges': [{'text': source}]})
                completion = client.request('textDocument/completion', dict(doc, position=position(source, needle, len(needle))))
                items = completion['items']
                labels = [item['label'] for item in items]
                assert all(name in labels for name in names), (needle, names, labels)
                if needle.endswith('(cho'):
                    assert all(name in labels for name in absent), (needle, absent, labels)
                    assert all(labels.index(names[0]) < labels.index(name) for name in absent), (needle, names, absent, labels)
                else:
                    assert all(name not in labels for name in absent), (needle, names, absent, labels)
                assert client.request('textDocument/semanticTokens/full', doc)['data']
        finally:
            client.close()
        log.seek(0)
        logs = log.read()
        assert 'panic' not in logs.lower() and 'failed to implement' not in logs, logs

    for needle, offset, expected_construct in [
        ('data.Number?', len('data.Number'), 'optional-type'),
    ]:
        point = position(MAIN, needle, offset)
        target = 'main.go:%d:%d' % (point['line']+1, point['character']+1)
        query = json.loads(command(GON, 'query', 'type', target, '--json', cwd=folder, env=ENV))
        assert query['results'][0]['type']['construct'] == expected_construct, query

print('PASS: optional syntax baseline/legacy/modern execution, LSP diagnostics, imported named results, native presence bindings/references/rename, tokens, formatting, signatures, optional completion, CLI types')
