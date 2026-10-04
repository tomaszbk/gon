#!/usr/bin/env python3
"""Focused real LSP and executable named-argument compatibility checks."""
import json
import os
from pathlib import Path
import tempfile
from test import Client, GON, command, position, apply_edits

BASELINE = os.environ.get('GON_BASELINE_GO')
if not BASELINE:
    raise SystemExit('Set GON_BASELINE_GO to the unmodified Go 1.27+ baseline')

LIB = '''package pair
func Pair(first, second int) int { return first*10 + second }
func Generic[T any](first T, second []T) T { return first }
'''
MODERN = '''package main
import ("fmt"; "example.com/named/pair")
func main() {
    chosenInt, chosenString := 1, "unused"
    _ = chosenString
    var f func(left, right int) int = pair.Pair
    fmt.Println(pair.Pair(second: 2, first: chosenInt), f(right: 4, left: 3))
    _ = pair.Generic[int](second: []int{2}, first: chosenInt)
}
'''
LEGACY = MODERN.replace('pair.Pair(second: 2, first: chosenInt)', 'pair.Pair(chosenInt, 2)').replace('f(right: 4, left: 3)', 'f(3, 4)').replace('pair.Generic[int](second: []int{2}, first: chosenInt)', 'pair.Generic[int](chosenInt, []int{2})')

with tempfile.TemporaryDirectory(prefix='gon-named-editor-') as temp:
    folder = Path(temp).resolve()
    (folder / 'go.mod').write_text('module example.com/named\n\ngo 1.27\n')
    lib = folder / 'pair' / 'pair.go'
    lib.parent.mkdir()
    lib.write_text(LIB)
    file = folder / 'main.go'
    file.write_text(LEGACY)
    assert command(BASELINE, 'run', '.', cwd=folder) == '12 34\n'
    assert command(GON, 'run', '.', cwd=folder) == '12 34\n'
    file.write_text(MODERN)
    assert command(GON, 'run', '.', cwd=folder) == '12 34\n'
    uri = file.as_uri()
    doc = {'textDocument': {'uri': uri}}
    with (folder / 'lsp.log').open('w+') as log:
        client = Client(folder, log)
        try:
            client.send('textDocument/didOpen', {'textDocument': {'uri': uri, 'languageId': 'gon', 'version': 1, 'text': MODERN}})
            diagnostics = client.diagnostics(uri, 1, lambda d: True)
            assert not any(d.get('severity') == 1 for d in diagnostics), diagnostics
            label = position(MODERN, 'second: 2')
            definitions = client.request('textDocument/definition', dict(doc, position=label))
            assert definitions[0]['uri'] == lib.as_uri(), definitions
            assert definitions[0]['range']['start'] == position(LIB, 'second int'), definitions
            signature = client.request('textDocument/signatureHelp', dict(doc, position=position(MODERN, 'second: 2', len('second: '))))
            assert signature['signatures'][0]['activeParameter'] == 1, signature
            tokens = client.request('textDocument/semanticTokens/full', doc)
            assert tokens['data'], tokens
            refs = client.request('textDocument/references', {'textDocument': {'uri': lib.as_uri()}, 'position': position(LIB, 'first, second'), 'context': {'includeDeclaration': True}})
            assert any(ref['uri'] == uri and ref['range']['start'] == position(MODERN, 'first: chosenInt') for ref in refs), refs
            renamed = client.request('textDocument/rename', {'textDocument': {'uri': lib.as_uri()}, 'position': position(LIB, 'first, second'), 'newName': 'start'})
            changes = {change['textDocument']['uri']: change['edits'] for change in renamed['documentChanges']}
            assert uri in changes and lib.as_uri() in changes, renamed
            newmain = apply_edits(MODERN, changes[uri])
            assert 'start: chosenInt' in newmain and 'left: 3' in newmain, newmain
            lib.write_text(apply_edits(LIB, changes[lib.as_uri()]))
            file.write_text(newmain)
            assert command(GON, 'run', '.', cwd=folder) == '12 34\n'
            lib.write_text(LIB)
            file.write_text(MODERN)
            for version, (old, new, needle, expected, absent) in enumerate([
                ('second: 2, first: chosenInt', 'se, first: chosenInt', 'se, first:', 'second:', 'first:'),
                ('second: 2, first: chosenInt', '1, se', '1, se', 'second:', 'first:'),
                ('second: 2, first: chosenInt', 'second: cho, first: chosenInt', 'second: cho', 'chosenInt', 'second:'),
            ], 2):
                source = MODERN.replace(old, new)
                client.send('textDocument/didChange', dict(doc, textDocument={'uri': uri, 'version': version}, contentChanges=[{'text': source}]))
                offset = len('se') if needle.startswith('se,') else len(needle)
                completion = client.request('textDocument/completion', dict(doc, position=position(source, needle, offset)))
                labels = [item['label'] for item in completion['items']]
                assert expected in labels and absent not in labels, (expected, absent, labels)
        finally:
            client.close()
        log.seek(0)
        logs = log.read()
        assert 'panic' not in logs.lower() and 'failed to implement' not in logs, logs
print('PASS: named argument baseline/legacy/modern execution, LSP definition/references/rename/signature/completion')
