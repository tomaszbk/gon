# Maintained Gon language server

This directory is the maintained Gon adaptation of gopls. Edit these sources
directly. It is a separate Go module within the Gon repository, built with Gon's
compiler, public parser, AST, types, and formatter. Its upstream module path is
retained for internal imports; the public executable remains `gonpls`.

The initial source import is gopls v0.23.0 from `golang.org/x/tools/gopls`.
[`UPSTREAM.json`](UPSTREAM.json) records the module and go.mod checksums, tag,
repository, and commit. Both checksums were verified using `go mod download`
before copying source. The complete existing `misc/gon/patches/gopls.patch`,
including the pending compiler-test workspace fix, was applied during import.
That patch was then removed; future changes belong here. The upstream license,
tests, documentation, module requirements, and checksums are retained.

From the Gon repository root, after building the compiler:

```sh
python3 misc/gon/build.py
./gon/bin/gonpls version
cd tools/gonpls
../../gon/bin/gon test ./internal/protocol
../../gon/bin/gon test ./internal/cache -run 'TestGonCompilerTest|TestStandalone'
../../gon/bin/gon test ./internal/cmd
```

The build uses the maintained sibling modules `../x-tools` and `../staticcheck`
through local replacements. Vet uses that same x/tools source through a generated
vendor tree. No dependency patches or `pkg/gon-tools` copies are used. Baseline
provenance lives in each module's `UPSTREAM.json`; adapted source is versioned
here. The server still builds with `-mod=readonly`, `-trimpath` and
`-buildvcs=false`.

The x/tools upgrade on 2026-10-02 required adapting completion's standard-symbol
metadata, variable-kind access, the package-path heuristic and analyzers that
moved into vet.Suite. The Gon corpus test uses the actual analyzer registry.
The subsequent Go master integration updates x/tools to `98444708d405`;
receiver inspection now uses `typesinternal.RecvBase` for methods and
`Unpointer` plus `types.Unalias` for ordinary fields/results. This preserves
receiver aliases and pointer handling without restoring the removed helper.
See [integration gates](../../misc/gon/INTEGRATION.md).

## Gon adaptations

Carried over from the former patch:

- The `gonpls` name, `gonpls.*` command namespace and `gon` LSP language ID.
  Every server command and the `gonpls.doc.features` code action kind use the
  `gonpls.` prefix (see `internal/protocol/command`, where the generator and
  `TestNamespace` enforce it). The upstream `gopls.*` IDs are not accepted, so
  the official Go extension (`gopls.*`) and the Gon extension can be active in
  one VS Code window without registering the same command IDs.
- Semantic tokens for postfix `!` and `or` handlers.
- LSP cancellation codes instead of generic failures for cancelled requests.
- Standalone loading of compiler test inputs under the toolchain's `test` tree.
- No upstream telemetry uploads or crash reports.
- Predeclared named types have no package: `error`, `comparable` and Gon's
  `Result`. Method-set fingerprints encode them as a bare name (`Result`), never
  as a `(qual PATH NAME)` of a package-level type, so methods, interface methods
  and aliases mentioning `Result` are indexed and matched across packages. Code
  that reaches a `*types.Named` through `Obj().Pkg()` must handle nil, as in the
  test-function code lens and the implement-interface action; the inline-variable
  action skips named-argument labels and contextual `.Ok/.Err` names, which are
  not lexical references. Regression tests: `TestPredeclaredResult*`,
  `TestGonResult*` and the `inline-var-gon` marker test.

Added for the tooling commands of the public `gon` launcher:

- `internal/cmd/gon*.go`: `gon query`, `refactor`, `check`, `explain` and
  `capabilities`, reached as `gonpls gon ...` from `main.go` (see
  [misc/gon/CLI.md](../../misc/gon/CLI.md)). Each command drives a gonpls
  session in its own process through its LSP server and snapshots.
- `settings.InternalOptions.OnDemandDiagnostics`, which the command engine sets
  so that the server skips background diagnostics; editors are unaffected.
- No pkg.go.dev links for type-checker codes that upstream x/tools does not
  document, including `InvalidErrorHandling`; the maintained x/tools module names codes
  148–151 and Gon's own range starting at 10000.
- Upstream command-line test expectations and `internal/cmd/usage` help files
  updated for the `gonpls` name and command namespace.
- The cache, integration and marker test binaries ignore an ambient `GOWORK`
  (for example `GOWORK=off`); otherwise every `go.work` test runs as a `go.mod`
  or ad-hoc view and fails.

When updating the upstream baseline, start from the module identified by
`UPSTREAM.json`, compare upstream changes against this maintained tree, carry
forward the Gon adaptations, update provenance and pins, and run the real LSP
and executable compatibility regressions in `misc/gon`. Do not reintroduce a
second gopls patch containing the changes maintained here.
