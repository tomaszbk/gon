#!/usr/bin/env python3
"""Execute a string enum pair and exercise its real editor and semantic CLI."""
import json
import os
from pathlib import Path
import tempfile

from test import Client, GON, command, position, apply_edits

BASELINE = os.environ.get('GON_BASELINE_GO')
if not BASELINE:
    raise SystemExit('Set GON_BASELINE_GO to an unmodified Go 1.27+ baseline')

LIB = '''package data
const teacherText = "teacher"
// Role preserves unrecognized names.
type Role enum string {
    default Unknown(string)
    Teacher = teacherText
    Student = "student"
}
'''
LEGACY_LIB = '''package data
const teacherText = "teacher"
type Role string
func ParseRole(text string) Role { return Role(text) }
func (value Role) String() string { return string(value) }
'''
LEGACY = '''package main
import ("fmt"; "example.com/stringenums/data")
func main() {
    role := data.ParseRole("teacher")
    parse := data.ParseRole
    fmt.Println(role.String(), parse("student").String(), parse("visitor").String())
}
'''
MODERN = LEGACY.replace('data.ParseRole', 'data.Role.Parse')
EXPECTED = 'teacher student visitor\n'


def point_target(path, source, needle):
    point = position(source, needle)
    return f'{path}:{point["line"]+1}:{point["character"]+1}'


def query(folder, kind, target):
    result = json.loads(command(GON, 'query', kind, target, '--json', cwd=folder))
    assert result['ok'], result
    return result['results'][0]


with tempfile.TemporaryDirectory(prefix='gon-stringenum-editor-') as temp:
    folder = Path(temp).resolve()
    (folder / 'go.mod').write_text('module example.com/stringenums\n\ngo 1.27\n')
    lib = folder / 'data' / 'data.go'
    lib.parent.mkdir()
    main = folder / 'main.go'
    lib.write_text(LEGACY_LIB)
    main.write_text(LEGACY)
    assert command(BASELINE, 'run', '.', cwd=folder) == EXPECTED
    assert command(GON, 'run', '.', cwd=folder) == EXPECTED
    lib.write_text(LIB)
    main.write_text(MODERN)
    assert command(GON, 'run', '.', cwd=folder) == EXPECTED
    uri, liburi = main.as_uri(), lib.as_uri()
    doc = {'textDocument': {'uri': uri}}
    parse_point = position(MODERN, 'Parse("teacher")')
    string_point = position(MODERN, 'String()')
    with (folder / 'lsp.log').open('w+') as log:
        client = Client(folder, log, snippet_support=True)
        try:
            for fileuri, source in [(uri, MODERN), (liburi, LIB)]:
                client.send('textDocument/didOpen', {'textDocument': {'uri': fileuri, 'languageId': 'gon', 'version': 1, 'text': source}})
                diagnostics = client.diagnostics(fileuri, 1, lambda d: True)
                assert not any(d.get('severity') == 1 for d in diagnostics), diagnostics
            for point, signature in [(parse_point, 'Role.Parse(text string)'), (string_point, 'String() string')]:
                definitions = client.request('textDocument/definition', dict(doc, position=point))
                assert definitions[0]['uri'] == liburi, definitions
                assert definitions[0]['range']['start'] == position(LIB, 'Role enum'), definitions
                assert definitions[0]['range']['end']['character'] - definitions[0]['range']['start']['character'] == len('Role'), definitions
                hover = client.request('textDocument/hover', dict(doc, position=point))
                assert signature in hover['contents']['value'], hover
            refs = client.request('textDocument/references', dict(doc, position=parse_point, context={'includeDeclaration': False}))
            expected = [parse_point, position(MODERN, 'Parse\n')]
            assert len(refs) == 2 and all(ref['range']['start'] in expected for ref in refs), refs
            for point in [parse_point, string_point]:
                try:
                    client.request('textDocument/rename', dict(doc, position=point, newName='Changed'))
                    raise RuntimeError('accepted rename of an automatic string enum member')
                except AssertionError as error:
                    assert 'automatic string enum member' in str(error), error
            renamed = client.request('textDocument/rename', {'textDocument': {'uri': liburi}, 'position': position(LIB, 'teacherText ='), 'newName': 'teacherSpelling'})
            changes = {change['textDocument']['uri']: change['edits'] for change in renamed['documentChanges']}
            updated = apply_edits(LIB, changes[liburi])
            assert 'Teacher = teacherSpelling' in updated and 'const teacherSpelling =' in updated, updated
            lib.write_text(updated)
            assert command(GON, 'run', '.', cwd=folder) == EXPECTED
            lib.write_text(LIB)
            response = client.request('textDocument/semanticTokens/full', {'textDocument': {'uri': liburi}})
            kinds = client.result['capabilities']['semanticTokensProvider']['legend']['tokenTypes']
            tokens = {}
            line = column = 0
            for offset in range(0, len(response['data']), 5):
                dline, dcolumn, length, kind, _ = response['data'][offset:offset+5]
                column = dcolumn if dline else column + dcolumn
                line += dline
                tokens[(line, column)] = (length, kinds[kind])
            for needle, kind in [('enum string', 'keyword'), ('string {', 'keyword'), ('Teacher =', 'enumMember'), ('teacherText\n', 'variable')]:
                point = position(LIB, needle)
                actual = tokens.get((point['line'], point['character']))
                assert actual and actual[1] == kind, (needle, actual, tokens)
            for version, (old, new, needle, expected) in enumerate([
                ('data.Role.Parse("teacher")', 'data.Role.Par', 'data.Role.Par', ['Parse']),
                ('role.String()', 'role.Str', 'role.Str', ['String']),
                ('role.String()', 'role.Mar', 'role.Mar', ['MarshalText']),
                ('role.String()', 'role.Unm', 'role.Unm', ['UnmarshalText']),
            ], 2):
                source = MODERN.replace(old, new)
                client.send('textDocument/didChange', {'textDocument': {'uri': uri, 'version': version}, 'contentChanges': [{'text': source}]})
                completion = client.request('textDocument/completion', dict(doc, position=position(source, needle, len(needle))))
                labels = [item['label'] for item in completion['items']]
                assert all(name in labels for name in expected) and '$gonTag' not in labels, (expected, labels)
            malformed = MODERN + '\ntype Empty enum string {}\nvar malformed = Empty.Parse("x")\nfunc partial() { _ = Empty. }\n'
            client.send('textDocument/didChange', {'textDocument': {'uri': uri, 'version': 6}, 'contentChanges': [{'text': malformed}]})
            diagnostics = client.diagnostics(uri, 6, lambda d: any(item.get('severity') == 1 for item in d))
            assert any(item.get('severity') == 1 for item in diagnostics), diagnostics
            client.request('textDocument/completion', dict(doc, position=position(malformed, 'Empty. }', len('Empty.'))))
        finally:
            client.close()
        log.seek(0)
        logs = log.read()
        assert 'panic' not in logs.lower() and 'failed to implement' not in logs, logs
    parsed = query(folder, 'type', point_target('main.go', MODERN, 'Parse("teacher")'))
    assert 'func(text string)' in parsed['type']['type'] and 'Role' in parsed['type']['type'], parsed
    definition = query(folder, 'def', './data.Role.Parse')
    target = definition['target']
    assert target['object']['name'] == 'Parse', target
    assert definition['items'][0]['object']['name'] == 'Parse' and 'Role.Parse' in definition['items'][0]['hover'], definition
    assert query(folder, 'refs', './data.Role.Parse')['total'] == 3
    assert query(folder, 'refs', './data.Role.String')['total'] == 4
    checked = json.loads(command(GON, 'check', './...', '--json', cwd=folder))
    assert checked['ok'] and checked['summary']['errors'] == 0, checked

print('PASS: string enums baseline/legacy/modern execution; real LSP completion, definition, hover, references, rename, tokens; semantic CLI')
