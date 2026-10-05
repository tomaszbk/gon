#!/usr/bin/env python3
"""Real LSP enum/matching regressions, paired with executable compatibility."""
import os
from pathlib import Path
import tempfile
from test import Client, GON, command, position, apply_edits

BASELINE = os.environ.get('GON_BASELINE_GO')
if not BASELINE:
    raise SystemExit('Set GON_BASELINE_GO to the unmodified Go 1.27+ baseline')

LIB = '''package data
// Value carries one of three alternatives.
type Value enum {
    default Empty
    // Int carries the chosen integer.
    Int(int)
    Record { Count int; hidden bool }
    secret
}
type Maybe[T any] enum { default None; Some(T) }
func New() Value { return Value.Int(7) }
'''
LEGACY_LIB = '''package data
type Value struct { Kind int; Count int }
func New() Value { return Value{Kind: 1, Count: 7} }
'''
MODERN = '''package main
import ("fmt"; "example.com/alternatives/data")
func count(v data.Value) int {
    return switch v {
    case data.Value.Empty => 0
    case data.Value.Int(chosen) if chosen > 0 => chosen
    case data.Value.Int(other) => other
    case data.Value.Record { Count: total, ... } => total
    default => -1
    }
}
func extract(v data.Maybe[int]) int {
    return switch v {
    case data.Maybe[int].Some(chosen) => chosen
    case data.Maybe[int].None => 0
    }
}
func canonicalValues() {
    _ = (int?)(5)
    _ = (int?)(nil)
    _ = Result[int, string].Ok(6)
    _ = Result[int, string].Err("failure")
    var missing int?
    var present int? = -1
    var typedNil (*int)? = (*int)(nil)
    var values ([]int)? = ([]int)(nil)
    if missing != nil || nil == present || typedNil == nil || nil == values { panic("optional presence") }
}
func patternNames(b bool, p *int) int {
    const localValue = 99
    true, false, nil := 10, 20, 30
    chosen := switch b { case true => true; case false => false }
    pointer := switch ((*int)?)(p) {
    case nil? => nil
    case _? => 0
    case nil => -1
    }
    bound := switch data.Value.Int(7) {
    case data.Value.Int(localValue) => localValue
    default => 0
    }
    return chosen + pointer + bound + false + nil + localValue
}
func main() {
    canonicalValues()
    if patternNames(true, nil) != 196 { panic("pattern names") }
    record := data.Value.Record{Count: 9}
    fmt.Println(count(data.New()), count(record), extract(data.Maybe[int].Some(3)))
}
'''
LEGACY = '''package main
import ("fmt"; "example.com/alternatives/data")
func count(v data.Value) int {
    switch v.Kind { case 0: return 0; case 1, 2: return v.Count; default: return -1 }
}
func patternNames(b bool, p *int) int {
    var absent *int
    const localValue = 99
    true, false, nil := 10, 20, 30
    chosen := false
    if b { chosen = true }
    pointer := 0
    if p == absent { pointer = nil }
    bound := 0
    if value := data.New(); value.Kind == 1 {
        localValue := value.Count
        bound = localValue
    }
    return chosen + pointer + bound + false + nil + localValue
}
func main() {
    if patternNames(true, nil) != 196 { panic("pattern names") }
    fmt.Println(count(data.New()), count(data.Value{Kind: 2, Count: 9}), 3)
}
'''


def decoded_tokens(client, response):
    legend = client.result['capabilities']['semanticTokensProvider']['legend']['tokenTypes']
    line = char = 0
    result = {}
    data = response['data']
    for offset in range(0, len(data), 5):
        delta_line, delta_char, length, kind, modifiers = data[offset:offset+5]
        char = delta_char if delta_line else char + delta_char
        line += delta_line
        result[(line, char)] = (length, legend[kind])
    return result


with tempfile.TemporaryDirectory(prefix='gon-alternatives-editor-') as temp:
    folder = Path(temp).resolve()
    (folder / 'go.mod').write_text('module example.com/alternatives\n\ngo 1.27\n')
    lib = folder / 'data' / 'data.go'
    lib.parent.mkdir()
    main = folder / 'main.go'
    lib.write_text(LEGACY_LIB)
    main.write_text(LEGACY)
    assert command(BASELINE, 'run', '.', cwd=folder) == '7 9 3\n'
    assert command(GON, 'run', '.', cwd=folder) == '7 9 3\n'
    lib.write_text(LIB)
    main.write_text(MODERN)
    assert command(GON, 'run', '.', cwd=folder) == '7 9 3\n'
    uri, liburi = main.as_uri(), lib.as_uri()
    doc = {'textDocument': {'uri': uri}}
    with (folder / 'lsp.log').open('w+') as log:
        client = Client(folder, log, snippet_support=True)
        try:
            for fileuri, source in [(uri, MODERN), (liburi, LIB)]:
                client.send('textDocument/didOpen', {'textDocument': {'uri': fileuri, 'languageId': 'gon', 'version': 1, 'text': source}})
                diagnostics = client.diagnostics(fileuri, 1, lambda d: True)
                assert not any(d.get('severity') == 1 for d in diagnostics), diagnostics
            variant = position(MODERN, 'Int(chosen)')
            definitions = client.request('textDocument/definition', dict(doc, position=variant))
            assert definitions[0]['uri'] == liburi, definitions
            assert definitions[0]['range']['start'] == position(LIB, 'Int(int)'), definitions
            hover = client.request('textDocument/hover', dict(doc, position=variant))
            assert 'Int(int)' in hover['contents']['value'] and 'chosen integer' in hover['contents']['value'], hover
            unit = position(MODERN, 'Empty =>')
            definitions = client.request('textDocument/definition', dict(doc, position=unit))
            assert definitions[0]['uri'] == liburi and definitions[0]['range']['start'] == position(LIB, 'Empty\n'), definitions
            hover = client.request('textDocument/hover', dict(doc, position=unit))
            assert 'Value.Empty' in hover['contents']['value'], hover
            for spelling, declaration, signature in [
                ('Ok(6)', 'Ok(T)', 'Result[int, string].Ok(int)'),
                ('Err("failure")', 'Err(E)', 'Result[int, string].Err(string)'),
            ]:
                point = position(MODERN, spelling)
                definitions = client.request('textDocument/definition', dict(doc, position=point))
                builtin = Path(__file__).resolve().parents[2] / 'src' / 'builtin' / 'builtin.go'
                assert definitions[0]['uri'].endswith('/src/builtin/builtin.go'), definitions
                assert definitions[0]['range']['start'] == position(builtin.read_text(), declaration), definitions
                hover = client.request('textDocument/hover', dict(doc, position=point))
                assert signature in hover['contents']['value'], hover
            binder = position(MODERN, 'chosen)')
            use = position(MODERN, 'chosen >')
            definitions = client.request('textDocument/definition', dict(doc, position=use))
            assert definitions[0]['range']['start'] == binder, definitions
            hover = client.request('textDocument/hover', dict(doc, position=use))
            assert 'chosen int' in hover['contents']['value'], hover
            refs = client.request('textDocument/references', dict(doc, position=binder, context={'includeDeclaration': True}))
            expected = [binder, use, position(MODERN, '=> chosen', len('=> '))]
            assert len(refs) == 3 and all(ref['range']['start'] in expected for ref in refs), refs
            renamed = client.request('textDocument/rename', dict(doc, position=binder, newName='selected'))
            changes = {change['textDocument']['uri']: change['edits'] for change in renamed['documentChanges']}
            updated = apply_edits(MODERN, changes[uri])
            assert 'Int(selected) if selected > 0 => selected' in updated and 'Some(chosen) => chosen' in updated, updated
            main.write_text(updated)
            assert command(GON, 'run', '.', cwd=folder) == '7 9 3\n'
            main.write_text(MODERN)
            # Contextual pattern values retain their predeclared identities;
            # ordinary Go names and payload bindings keep their own scopes.
            local_true = position(MODERN, 'true, false, nil :=')
            renamed = client.request('textDocument/rename', dict(doc, position=local_true, newName='yes'))
            changes = {change['textDocument']['uri']: change['edits'] for change in renamed['documentChanges']}
            updated = apply_edits(MODERN, changes[uri])
            assert 'case true => yes' in updated and 'case false => false' in updated, updated
            main.write_text(updated)
            assert command(GON, 'run', '.', cwd=folder) == '7 9 3\n'
            main.write_text(MODERN)
            local_binding = position(MODERN, 'Int(localValue)', len('Int('))
            local_use = position(MODERN, '=> localValue', len('=> '))
            definitions = client.request('textDocument/definition', dict(doc, position=local_use))
            assert definitions[0]['range']['start'] == local_binding, definitions
            refs = client.request('textDocument/references', dict(doc, position=position(MODERN, 'localValue = 99'), context={'includeDeclaration': True}))
            assert len(refs) == 2 and all(ref['range']['start'] != local_binding for ref in refs), refs
            # Exported variant rename updates constructions and matching across packages.
            for declaration, newname in [('Int(int)', 'Number'), ('Some(T)', 'Present'), ('Count int', 'Total')]:
                renamed = client.request('textDocument/rename', {'textDocument': {'uri': liburi}, 'position': position(LIB, declaration), 'newName': newname})
                changes = {change['textDocument']['uri']: change['edits'] for change in renamed['documentChanges']}
                assert uri in changes and liburi in changes, renamed
                main.write_text(apply_edits(MODERN, changes[uri]))
                lib.write_text(apply_edits(LIB, changes[liburi]))
                assert command(GON, 'run', '.', cwd=folder) == '7 9 3\n'
                main.write_text(MODERN)
                lib.write_text(LIB)
            for fileuri, source, needles in [
                (uri, MODERN, [('switch v', 'keyword'), ('case data', 'keyword'), ('=> 0', 'operator'), ('chosen)', 'variable'), ('Count: total', 'property'), ('Int(chosen)', 'enumMember'), ('Some(chosen)', 'enumMember')]),
                (liburi, LIB, [('enum {', 'keyword'), ('default Empty', 'keyword'), ('Empty\n', 'enumMember')]),
            ]:
                tokens = decoded_tokens(client, client.request('textDocument/semanticTokens/full', {'textDocument': {'uri': fileuri}}))
                for needle, kind in needles:
                    pos = position(source, needle)
                    actual = tokens.get((pos['line'], pos['character']))
                    assert actual and actual[1] == kind, (needle, kind, actual, tokens)
            for version, (old, new, needle, names, absent) in enumerate([
                ('data.Value.Record{Count: 9}', 'data.Value.R', 'record := data.Value.R', ['Record'], ['secret', '$gonTag']),
                ('data.Value.Record{Count: 9}', 'data.Value.I', 'record := data.Value.I', ['Int'], ['secret', '$gonTag']),
                ('data.Maybe[int].Some(3)', 'data.Maybe[int].S', 'extract(data.Maybe[int].S', ['Some'], ['secret']),
                ('data.Value.Record{Count: 9}', 'data.Value.Record{Co}', 'Record{Co', ['Count'], ['hidden', '$gonTag']),
                ('chosen > 0', 'cho > 0', 'if cho', ['chosen'], ['other', 'total']),
                ('Count: total', 'Co: total', 'Co: total', ['Count'], ['hidden', '$gonTag']),
                ('case data.Value.Record { Count: total, ... }', 'case data.Value.Re', 'case data.Value.Re', ['Record'], ['secret', '$gonTag']),
                ('data.Value.Record{Count: 9}', 'data.Value.Re{Count: 9}', 'record := data.Value.Re', ['Record'], ['secret']),
            ], 2):
                source = MODERN.replace(old, new)
                client.send('textDocument/didChange', {'textDocument': {'uri': uri, 'version': version}, 'contentChanges': [{'text': source}]})
                completion = client.request('textDocument/completion', dict(doc, position=position(source, needle, 2 if needle == 'Co: total' else len(needle))))
                items = completion['items']
                labels = [item['label'] for item in items]
                assert all(name in labels for name in names) and all(name not in labels for name in absent), (new, needle, names, absent, labels)
                if names == ['Record']:
                    item = next(item for item in items if item['label'] == 'Record')
                    insert = item.get('textEdit', {}).get('newText', item.get('insertText', ''))
                    if new.endswith('{Count: 9}'):
                        assert insert == 'Record', item
                    else:
                        assert '{Count:' in insert and 'hidden:' not in insert, item
                        if needle.startswith('case '):
                            assert '...' in insert, item
        finally:
            client.close()
        log.seek(0)
        logs = log.read()
        assert 'panic' not in logs.lower() and 'failed to implement' not in logs, logs
print('PASS: alternatives baseline/legacy/modern execution, real LSP constructors/fields/definition/hover/binding scopes/rename/tokens')
