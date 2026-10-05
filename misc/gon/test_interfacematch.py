#!/usr/bin/env python3
"""Real LSP regressions for enum variant patterns on interface subjects.

The paired legacy program searches the error tree with errors.As; the modern
program matches error values directly. Both run with the baseline and Gon."""
import os
from pathlib import Path
import tempfile
from test import Client, GON, command, position, apply_edits

BASELINE = os.environ.get('GON_BASELINE_GO')
if not BASELINE:
    raise SystemExit('Set GON_BASELINE_GO to the unmodified Go 1.27+ baseline')

LIB = '''package data
// Failure reports why a request failed.
type Failure enum {
    default Unknown
    // Rejected carries an HTTP status and a message.
    Rejected { Status int; Message string }
    Retry(int)
}
func (Failure) Error() string { return "failure" }
func Reject(status int, message string) error { return Failure.Rejected{Status: status, Message: message} }
func Retry(attempt int) error { return Failure.Retry(attempt) }
'''
LEGACY_LIB = '''package data
import "errors"
type Failure struct { kind, status, attempt int; message string }
func (Failure) Error() string { return "failure" }
func Reject(status int, message string) error { return Failure{kind: 1, status: status, message: message} }
func Retry(attempt int) error { return Failure{kind: 2, attempt: attempt} }
func Rejected(err error) (int, string, bool) {
    var f Failure
    if errors.As(err, &f) && f.kind == 1 { return f.status, f.message, true }
    return 0, "", false
}
func Retrying(err error) (int, bool) {
    var f Failure
    if errors.As(err, &f) && f.kind == 2 { return f.attempt, true }
    return 0, false
}
'''
MODERN = '''package main
import ("errors"; "fmt"; "example.com/interfacematch/data")
var errBoom = errors.New("boom")
func describe(err error) string {
    return switch err {
    case data.Failure.Rejected{Status: status, Message: message} => fmt.Sprintf("%d %s", status, message)
    case data.Failure.Retry(attempt) if attempt > 1 => fmt.Sprint("retry ", attempt)
    case data.Failure.Retry(_) => "retry once"
    case _ if errors.Is(err, errBoom) => "boom"
    default => "other"
    }
}
func main() {
    for _, err := range []error{data.Reject(404, "missing"), fmt.Errorf("wrapped: %w", data.Retry(3)), data.Retry(1), fmt.Errorf("w: %w", errBoom), errors.New("x"), nil} {
        fmt.Println(describe(err))
    }
}
'''
LEGACY = '''package main
import ("errors"; "fmt"; "example.com/interfacematch/data")
var errBoom = errors.New("boom")
func describe(err error) string {
    if status, message, ok := data.Rejected(err); ok { return fmt.Sprintf("%d %s", status, message) }
    if attempt, ok := data.Retrying(err); ok && attempt > 1 { return fmt.Sprint("retry ", attempt) }
    if _, ok := data.Retrying(err); ok { return "retry once" }
    if errors.Is(err, errBoom) { return "boom" }
    return "other"
}
func main() {
    for _, err := range []error{data.Reject(404, "missing"), fmt.Errorf("wrapped: %w", data.Retry(3)), data.Retry(1), fmt.Errorf("w: %w", errBoom), errors.New("x"), nil} {
        fmt.Println(describe(err))
    }
}
'''
OUTPUT = '404 missing\nretry 3\nretry once\nboom\nother\nother\n'


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


with tempfile.TemporaryDirectory(prefix='gon-interfacematch-editor-') as temp:
    folder = Path(temp).resolve()
    (folder / 'go.mod').write_text('module example.com/interfacematch\n\ngo 1.27\n')
    lib = folder / 'data' / 'data.go'
    lib.parent.mkdir()
    main = folder / 'main.go'
    lib.write_text(LEGACY_LIB)
    main.write_text(LEGACY)
    assert command(BASELINE, 'run', '.', cwd=folder) == OUTPUT
    assert command(GON, 'run', '.', cwd=folder) == OUTPUT
    lib.write_text(LIB)
    main.write_text(MODERN)
    assert command(GON, 'run', '.', cwd=folder) == OUTPUT
    uri, liburi = main.as_uri(), lib.as_uri()
    doc = {'textDocument': {'uri': uri}}
    with (folder / 'lsp.log').open('w+') as log:
        client = Client(folder, log, snippet_support=True)
        try:
            for fileuri, source in [(uri, MODERN), (liburi, LIB)]:
                client.send('textDocument/didOpen', {'textDocument': {'uri': fileuri, 'languageId': 'gon', 'version': 1, 'text': source}})
                diagnostics = client.diagnostics(fileuri, 1, lambda d: True)
                assert not any(d.get('severity') == 1 for d in diagnostics), diagnostics
            # A variant pattern on an error subject resolves to the declaration.
            variant = position(MODERN, 'Rejected{Status')
            definitions = client.request('textDocument/definition', dict(doc, position=variant))
            assert definitions[0]['uri'] == liburi and definitions[0]['range']['start'] == position(LIB, 'Rejected {'), definitions
            hover = client.request('textDocument/hover', dict(doc, position=variant))
            assert 'Rejected' in hover['contents']['value'] and 'HTTP status' in hover['contents']['value'], hover
            qualifier = position(MODERN, 'data.Failure.Rejected', len('data.'))
            definitions = client.request('textDocument/definition', dict(doc, position=qualifier))
            assert definitions[0]['uri'] == liburi and definitions[0]['range']['start'] == position(LIB, 'Failure enum', 0), definitions
            label = position(MODERN, 'Status: status')
            definitions = client.request('textDocument/definition', dict(doc, position=label))
            assert definitions[0]['uri'] == liburi and definitions[0]['range']['start'] == position(LIB, 'Status int'), definitions
            binder = position(MODERN, 'attempt) if')
            use = position(MODERN, 'attempt >')
            definitions = client.request('textDocument/definition', dict(doc, position=use))
            assert definitions[0]['range']['start'] == binder, definitions
            hover = client.request('textDocument/hover', dict(doc, position=use))
            assert 'attempt int' in hover['contents']['value'], hover
            refs = client.request('textDocument/references', dict(doc, position=binder, context={'includeDeclaration': True}))
            assert len(refs) == 3, refs
            tokens = decoded_tokens(client, client.request('textDocument/semanticTokens/full', {'textDocument': {'uri': uri}}))
            pos = position(MODERN, 'Rejected{Status')
            actual = tokens.get((pos['line'], pos['character']))
            assert actual and actual[1] == 'enumMember', (actual, tokens)
            # Renaming exported declarations updates patterns in another package.
            for declaration, newname in [('Retry(int)', 'Again'), ('Status int', 'Code')]:
                renamed = client.request('textDocument/rename', {'textDocument': {'uri': liburi}, 'position': position(LIB, declaration), 'newName': newname})
                changes = {change['textDocument']['uri']: change['edits'] for change in renamed['documentChanges']}
                assert uri in changes and liburi in changes, renamed
                main.write_text(apply_edits(MODERN, changes[uri]))
                lib.write_text(apply_edits(LIB, changes[liburi]))
                assert command(GON, 'run', '.', cwd=folder) == OUTPUT
                main.write_text(MODERN)
                lib.write_text(LIB)
            # Invalid patterns are diagnosed while editing.
            version = 1
            for old, new, messages in [
                ('    default => "other"\n', '', ['non-exhaustive match: missing _', 'never cover the interface type error']),
                ('func main()', 'type local enum { default A }\nfunc extra(err error) string { return switch err { case local.A => "a"; default => "d" } }\nfunc main()', ['local does not implement error']),
                ('case data.Failure.Retry(_) => "retry once"', 'case data.Failure.Retry(_) => "retry once"\n    case data.Failure.Retry(_) => "again"', ['unreachable match arm']),
                ('case data.Failure.Retry(_) => "retry once"', 'case data.Failure.Missing => "missing"', ['unknown or inaccessible alternative Missing']),
            ]:
                version += 1
                source = MODERN.replace(old, new)
                client.send('textDocument/didChange', {'textDocument': {'uri': uri, 'version': version}, 'contentChanges': [{'text': source}]})
                diagnostics = client.diagnostics(uri, version, lambda d: len(d) > 0)
                text = '\n'.join(d['message'] for d in diagnostics)
                assert all(message in text for message in messages), (new, messages, text)
            # Completion offers the qualified variants of an enum in a pattern.
            for old, new, needle, names, absent in [
                ('case data.Failure.Rejected{Status: status, Message: message}', 'case data.Failure.R', 'case data.Failure.R', ['Rejected', 'Retry'], ['Unknown']),
                ('case data.Failure.Rejected{Status: status, Message: message}', 'case data.Failure.Rejected{Sta}', 'Rejected{Sta', ['Status'], ['Message']),
            ]:
                version += 1
                source = MODERN.replace(old, new)
                client.send('textDocument/didChange', {'textDocument': {'uri': uri, 'version': version}, 'contentChanges': [{'text': source}]})
                completion = client.request('textDocument/completion', dict(doc, position=position(source, needle, len(needle))))
                labels = [item['label'] for item in completion['items']]
                assert all(name in labels for name in names) and all(name not in labels for name in absent), (new, names, absent, labels)
        finally:
            client.close()
        log.seek(0)
        logs = log.read()
        assert 'panic' not in logs.lower() and 'failed to implement' not in logs, logs
print('PASS: interface-subject variant patterns baseline/legacy/modern execution, real LSP definition/hover/references/rename/tokens/diagnostics/completion')
