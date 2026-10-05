# Current Gon validation

Current source target: **Gon 2.27**. Unmodified baseline:
`/opt/homebrew/bin/go`, **Go 1.27.1**. Native execution is **darwin/arm64**,
Apple M4. The fork's go1.28-devel version records source provenance. This file
records current evidence in place; no full release pass on every platform is claimed.

## Complete focused gates

```sh
GON_BASELINE_GO=/opt/homebrew/bin/go python3 misc/gon/validate.py enums
GON_BASELINE_GO=/opt/homebrew/bin/go python3 misc/gon/validate.py tooling
```

| Profile | Result | Sum of check durations | Evidence |
| --- | --- | ---: | --- |
| enums | 32/32 PASS, exit 0, no pending items | 98.123s | [summary](../../pkg/gon-validation/enums/summary.json) |
| tooling | 41/41 PASS, exit 0, no pending items | 147.654s | [summary](../../pkg/gon-validation/tooling/summary.json) |

These counts predate the 2026-10-05 merge recorded [below](#features-merged-on-2026-10-05):
the `tooling`, `errorhandling`, `matching` and `option` profiles have gained
checks since, and neither complete gate was rerun afterwards.

The enums gate includes the opt-in string-enum extension and ordinary enum
compatibility. String-enum pairs execute the Go 1.27.1 baseline, Gon legacy and
Gon modern with/without inlining and the JSON-v2 experiment. Assertions cover
known/unknown/empty text, zero, null, invalid JSON, escaping/UTF-8, independent
bytes, aliases, generic/local types, derived and imported types, interface and
method calls, written evaluation order, defer and goroutines. Native/SSA/IR
checks include local enums with their own type parameters inside generic
functions/methods. SSA interpretation and IR sanity checks also cover constraints
using the enclosing type parameter. Compile
rejections and real LSP tests cover malformed/incomplete declarations without
panic. Source and indexed/unified exports retain labels and generated methods.

The benchmark record separately retains the 47-check
[modern core gate](benchmarks/results/20261004T013816Z/diagnostics/modern/summary.json),
which predates the string-enum extension. Modern deduplicates enums, matching,
optional values, Result and named arguments.
Tooling executes the focused legacy/modern harnesses for all nine additions,
public AST, analyzer registry, export readers, CFG/SSA, Staticcheck IR, semantic
CLI, real LSP, modernization, README fixtures and benchmark correctness. Optional
checks include native identity, one-layer explicit/implicit conversions, typed
nil, nested presence patterns, shadowing, invalid contexts and metadata roundtrips.
Generic completion, implementation unification and method fingerprints retain
native optional identity and correctly discover/infer payload type parameters.

The migration test uses real CLI package loading, a default preview, a multi-file
revision-checked plan, atomic stale rejection, user-defined homonyms, nested
patterns, constructor function values, execution and idempotence. Normal
checking rejects retired constructors and migration does not modify Universe.

Compiler SSA executes 367 tests with no skips. The existing upstream
`internal/typesinternal/TestErrorCodes` skip remains; Gon error-code inventory
coverage executes. Per-check logs retain actual tests/skips. Compiler optimizations
cover tag loads at memory merges, integer Phis and unused rematerializations.
Focused tests check widths, aliases, overlap, unknown writes and live operands.

Legacy programs execute with Gon and Go 1.27.1; modern programs execute with Gon,
including non-inlined cases. Assertions cover success, failure, absence, typed
nil, nested layers, copied pointers, lazy defaults, evaluation order, named results
and defers. Relevant cgo/coverage pairs also execute. These are focused checks,
not whole-distribution test runs.

## SQL integration

The general `database/sql` boundary converts native string-enum arguments into
strings and parses string/byte results. There are no enum-specific SQL methods
or driver changes. The complete `database/sql` and `database/sql/driver` suites
passed **444 tests with no skips**, including existing Go behavior and the new
opt-in coverage. Both complete gates include these suites and
`test/stringenums_sql.go`: Go 1.27.1 legacy, Gon legacy, Gon modern and modern
with compiler inlining disabled produce identical observable results.

The deterministic pair exercises four driver paths: the old driver API,
`NamedValueChecker` returning `ErrSkip`, an accept-any checker bypassing default
conversion, and Go 1.27 `RowsColumnScanner`. Assertions cover known/unknown/empty
text, aliases, generics, detached byte buffers, nullable pointers and
`sql.Null[Role]`, named parameters, prepared statements and transactions.
Explicit `Scanner`/`Valuer` methods retain precedence. Plain enums and ordinary
Go text marshalers retain their existing SQL behavior. Nontext sources and NULL
fail without changing a direct enum destination; indirect pointer allocation
retains existing `database/sql` behavior.

```sh
GON_SQL_POSTGRES=1 GON_BASELINE_GO=/opt/homebrew/bin/go \
  python3 misc/gon/test_stringenums_postgres.py
```

This optional integration passed against **PostgreSQL 17.11**, using
**pgx stdlib v5.7.6** and **lib/pq v1.10.9**. Each driver executed the baseline,
Gon legacy, Gon modern and modern without inlining: **eight driver executions**
passed, including SQL NULL, prepared statements, transactions and multiple rows.
Client execution was darwin/arm64. The isolated Docker container used temporary
storage and was removed successfully. [Commands, versions and results](../../pkg/gon-validation/sql-postgres/summary.json)
retain the evidence. Setting `GON_SQL_POSTGRES=1` adds this integration to either
complete gate; the profile counts above are the default gates and record the
live integration separately. pgx's native API remains a separate adapter
boundary. This PostgreSQL run covers string enums only; the optional
integration below was not executed.

## Features merged on 2026-10-05

Master `96ab86645a` merges six branches: `database/sql` struct scanning, the
named-argument untyped-value fix, vet/gonpls tooling fixes, native optional
JSON and `database/sql`, enum patterns on interface and error subjects, and `!`
in tests and across Go tuples and Result. Every check below executed on
**darwin/arm64** (Apple M4); the one additional js/wasm execution, of the
`matchinterface` pair, is in the platform table. They are focused test
selections and partial `validate.py` runs, with the executable pairs executing
their legacy program on the unmodified baseline (`GON_BASELINE_GO`) and the
modern program with Gon, also without inlining. No full profile pass is
claimed: the `tooling`, `errorhandling`, `matching`, `option` and `modern`
profiles were not run in full after the merge, and the complete-gate counts
above predate it.

| Feature (commit) | Checks executed, all passing |
| --- | --- |
| `database/sql` struct scanning (`cb5b4dad4e`) | `database/sql` unit tests over the fakedb, basic, default and scancols drivers; `test/sqlstruct.go` pair; `cmd/api` check |
| Named-argument untyped values (`3aa69b804f`) | `TestNamedArgumentsRecordedTypes` in `types2` and `go/types`; `test/namedarguments.go` pair, extended with three invalid untyped cases |
| Vet and gonpls fixes (`8ca2c0bcc2`) | analyzer `TestGon` in `httpresponse` and `sqlrowserr`; `TestPredeclaredResult` (fingerprint), `TestPredeclaredResultMethods` (methodsets), `TestGonResult*` (LSP integration), marker `inline-var-gon`, cmd `TestGonResultMethods`; `validate.py tooling --only vet-error-handlers --only predeclared-result-index --only predeclared-result-lsp --only inline-variable-gon` (partial selection) |
| Native optional JSON and SQL (`f602bf641e`) | unit tests in `encoding/json` (default and `GOEXPERIMENT=nojsonv2`), `encoding/json/v2`, `database/sql`, `database/sql/driver` and `reflect`; `test/optionaljson.go` and `test/optionalsql.go` pairs; `validate.py option` partial selection: `optional-json`, `optional-json-v1`, `optional-sql`, `optional-reflect`, the two `vet` checks and the pair executions |
| Enum patterns on interface and error subjects (`2d8c28ed91`) | `TestMatchInterfaceSubject` and `TestMatchInterfaceSubjectInvalid` in both checkers; `test/matchinterface.go` pair; SSA and Staticcheck IR tests; `test_interfacematch.py` LSP test; `validate.py matching` partial selection including `matchinterface-execution`, `matchinterface-vet` and `interfacematch-lsp` |
| `!` in tests and across tuples and Result (`99c7b28518`) | `internal/types/testdata/check/errorbridge.go`; `TestErrorHandlingBoundaries` (56 cases) in both checkers; `test/errorbridge.go` and `test/errortest.go` pairs; `errors.TestErrNilResult`; SSA and Staticcheck IR tests; `validate.py errorhandling` partial selection (vendor, the `errorhandling`, `errorbridge` and `errortest` executions, types, syntax, ssa, staticcheck-ir, vet, cgo, cover, structural-tools, analyzers, compiler-ssa, compiler-inline, staticcheck-safety, refactor-safety) and `validate.py tooling` partial selection (tooling-api, syntax-fixes, fix-execution, cli, lsp) |

Integration on the merged tree (`96ab86645a`):

```sh
python3 misc/gon/vendor.py --check
go test -run 'TestGenerate|ErrorHandling|ErrorExpr|NamedArguments|OptionResult|MatchInterface' cmd/compile/internal/types2 go/types
go test -run 'ScanStruct|Collect|GonOptional|TestErrNilResult|^TestOptional' database/sql database/sql/driver encoding/json encoding/json/v2 errors reflect
go test cmd/internal/testdir -run 'Test/(sqlstruct|optionaljson|optionalsql|matchinterface|errorbridge|errortest|namedarguments|errorhandling|optionresult|matching|stringenums_sql)\.go$'
```

All four passed. A scratch module that combined `Collect` into `T?` fields,
tuple-to-Result and Result-to-error `!`, test `!`, an error-subject match and a
named comparison also passed `gon vet`, `gon test` and `gon check`.

Not executed: `misc/gon/test_optionals_postgres.py` (`GON_SQL_POSTGRES=1`) was
written but has not been run, so optional JSON and SQL have no live PostgreSQL
or driver evidence beyond `database/sql`'s own fake drivers. The new pairs
`sqlstruct`, `optionaljson`, `optionalsql` and `errorbridge` are registered in
`cross_pair.sh`, but their wasm and Linux runs are not part of this record;
`errortest` and `matchinterface` are not registered there. Benchmarks and
bootstrap were not rerun for these features.

## Bootstrap

The current string-enum compiler bootstrapped from unmodified Go 1.27.1 in the
repository, using a fresh build cache:

```sh
# Inside src; GOCACHE selects a fresh temporary directory:
env -u GOROOT -u GOTOOLDIR \
  GOROOT_BOOTSTRAP=/opt/homebrew/Cellar/go/1.27.1/libexec \
  GOMAXPROCS=1 GOFLAGS=-p=1 GOCACHE=<temporary-directory> ./make.bash
```

All three toolchains and command-staleness checks passed in **204.100s**.
[Commands and result](../../pkg/gon-validation/stringenums-final/bootstrap.json)
and [log](../../pkg/gon-validation/stringenums-final/bootstrap.log) retain the
evidence. An earlier shared-cache attempt failed the command-staleness check
for `internal/goarch`; the fresh-cache retry resolved it. The temporary cache
was removed, then `build.py` rebuilt the public tools and `vendor.py --check`
passed. This is a repository bootstrap, not an isolated source-only snapshot.

`make.bash` could also fail the command-staleness check intermittently (every
command stale on `internal/goarch`), for example on the 2026-10-05 Linux CI
runs. The cause was a nondeterministic object file, not the cache: package
`cmd/compile/internal/noder` has a noalg `[3]ir.Node` (the backing array of the
slice literal passed to `matchErrorAs`) and a regular one, which have identical
strings and share one type descriptor symbol, and `reflectdata.typesStrCmp`
left their order to the concurrent backend. The compiler built by toolchain2
then differed from the one built by toolchain3. `typesStrCmp` now writes the
type with algorithms first; `TestTypesStrCmpNoalg` covers it. On linux/arm64
(Debian, Go 1.27.1 bootstrap) the unfixed tree compiled `noder` to two different
objects in roughly half of identical runs, and the fixed tree gave one object in
24 of 24 runs with `-c=4`, `GOMAXPROCS=1` and `-c=1`, and three parallel
`make.bash` runs passed with the same compiler content ID.

The optional/Result benchmark record also includes an isolated source-only
snapshot bootstrapped from unmodified Go 1.27.1:

```sh
# Inside the isolated snapshot's src directory:
env -u GOROOT -u GOTOOLDIR \
  GOROOT_BOOTSTRAP=/opt/homebrew/Cellar/go/1.27.1/libexec \
  GOMAXPROCS=2 GOFLAGS=-p=2 ./make.bash
```

`make.bash` passed in **74.303s**. The resulting scratch compiler passed the
optional/Result and optional-syntax/boundary executable harnesses, plus a legacy
smoke with baseline-identical output. The snapshot contained 20,400 source files,
used APFS copy-on-write copies and a fresh cache, and excluded prebuilt tools.
Snapshot/cache were removed. Source-manifest SHA-256:
`f1d2c4b88b158bf750fce0be24c86a2a2f0109f03826bb8b3ee8693d577b23c1`.
[Runner, manifest, commands and logs](benchmarks/results/20261004T013816Z/diagnostics/bootstrap.json)
retain the evidence. Installing tools alone is not bootstrap evidence.

## Platforms

```sh
GON_BASELINE_GO=/opt/homebrew/bin/go misc/gon/cross_pair.sh js wasm
```

| Platform | Current evidence | Scope |
| --- | --- | --- |
| darwin/arm64 | Executed | Focused gates, baseline, bootstrap, benchmark, real LSP and editor; focused checks of the 2026-10-05 features |
| js/wasm | Executed through Node 26.6.0 | Eleven portable pairs, including string enums, SQL, analysis fixtures and local generic cases; 11 baseline and 33 Gon legacy/modern/non-inlined executions match |
| linux/amd64 | Cross-compiled | String-enum common/analysis and SQL pairs: baseline legacy, Gon legacy, Gon modern; existing optional/Result evidence retained below |
| linux/riscv64 | Cross-compiled | Same three variants and all three string-enum pairs |
| js/wasm (2026-10-05) | Executed through node | `matchinterface` pair only |
| linux/amd64, linux/riscv64, wasip1/wasm (2026-10-05) | Cross-compiled only | `matchinterface` pair only; not executed |

[Current wasm results](../../pkg/gon-validation/stringenums-cross/wasm.json)
passed in 5.829s; [12 string-enum cross-build commands/results](../../pkg/gon-validation/stringenums-cross/builds.json)
passed in 2.400s. The additional SQL pair's [four wasm executions](../../pkg/gon-validation/sql-cross/wasm.json)
passed in 2.905s and its [six Linux cross-builds](../../pkg/gon-validation/sql-cross/builds.json)
passed in 4.120s. Records include fixture hashes and logs. The benchmark record
also retains [optional wasm output](benchmarks/results/20261004T013816Z/diagnostics/wasm.log)
and [optional cross-build evidence](benchmarks/results/20261004T013816Z/diagnostics/cross-builds.json).
Linux target binaries were not executed locally; cross-compilation
is not runtime validation. Remote CI configuration is not execution evidence.
Target-specific cgo/editor behavior is not established on unexecuted targets.

## Performance

```sh
GON_BASELINE_GO=/opt/homebrew/bin/go python3 misc/gon/benchmark.py \
  --samples 10 --build-samples 7 --benchtime 300ms
```

The [current report](benchmarks/results/20261004T013816Z/report.md) uses native
optional fixtures and retains every one of the 24 workloads, including increases.
All 16 correctness tests pass per variant with identical observable output.
Each reported operation processes 64 items. Representation and retained-pointer
observations are separate from runtime timing; they do not promise a stable ABI
or zero overhead. Raw samples, hashes, source copies and commands are retained.
Published older experiments are removed. Exact costs and statistical limitations
are recorded in the report and [benchmark guide](benchmarks/README.md).

## Documentation and editor

`misc/gon/test_readme.py` passed in both complete gates: 28 displayed Go blocks
match 20 unique executed fixtures; legacy executes with Go 1.27.1/Gon and modern
with Gon. Adoption retains .go, go.mod/go.work, dependencies and ordinary Go
installations, selecting Gon explicitly for terminal/editor/CI. Installer
regressions execute as part of both profiles.

The string-enum vscode-gon update passed `npm run test:syntax`, `npm run compile`
and `npm run package` on macOS arm64 with VS Code 1.139.1. The actual TextMate
tokenizer covers ordinary/generic `enum string`, contextual identifiers and
comment/string isolation. VSIX inspection confirms twelve snippets, including
`gonstringenum`, and the updated grammar. The full extension-host suite was not
rerun for this update.

The separate vscode-gon repository's existing full `npm test` record uses
VS Code 1.139.1 and Delve 1.27.2. It tests native optional patterns, Result-only
contextual constructors, real client services, equivalent baseline/legacy/modern execution,
automatic discovery, per-folder overrides, Run/Check Project, debugger and test
explorer. Actual tokenizer and `npm run package` also passed. See that repository's
VALIDATION.md for the exact commands and platform limits.

## Supported limits

- Source inlining/extraction decline unsupported named, lazy, propagation,
  matching or target-sensitive moves. First-class enum constructors, string-enum
  parsers and generated string-enum methods disable compiler inlining.
- Optional storage is GC-safe but private, with discriminator/typed payload fields.
  Unit enums occupy 16 versus a legacy tag's 8 bytes; optional empty structs
  occupy 16 versus a legacy struct's 2 on this host. There is no stable ABI or
  automatic serialization/C mapping for ordinary enums or optional values;
  explicit adapters are required. Opt-in string enums supply text interfaces
  and standard JSON/`database/sql` conversions. SQL NULL uses `sql.Null[T]`,
  pointers or a native optional (the live PostgreSQL check for optionals is
  written but not executed). Native pgx outside `database/sql` needs a codec; `encoding/xml`,
  `gob` and C boundaries need explicit adapters.
- Native optional lifting is single-valued and one layer. Typed nil is present;
  nested layers and mixed optional/Go-nil guards require explicit boundaries.
- Result constructors need known targets. Go tuples and Result convert only at
  postfix `!`, because useful partial results may coexist with errors; every
  other tuple boundary needs explicit code. Enum patterns on an interface never
  prove exhaustiveness.
- The open items found during that merge are fixed in `2b0ee32fbc`. Checked on
  darwin/arm64: `go test go/build -run TestDependencies`; `go test reflect -run
  '^TestOptional|TestSplitEnumMetadata'`; `go test cmd/compile/internal/types2
  go/types -run 'TestStringEnum|^TestCheck$'` (full TestCheck in both checkers);
  `go test go/types -run Generate`; `go test ./go/types/objectpath` and
  `./go/analysis/passes/copylock` in tools/x-tools; `vendor.py --check`;
  `cmd/internal/testdir -run 'Test/(optionsyntax|optionresult|enums|stringenums|stringenums_sql|matching|matchinterface)\.go$'`
  with Go 1.27.1 as baseline.

## Upstream integration

[UPSTREAM.json](UPSTREAM.json) records integrated Go source revision
`67c1d421161d3d1ae9f5fd005e84c29fd0d9f896`. A three-way source comparison carried
Gon adaptations forward; upstream merge ancestry was not imported. Use that
revision as the previous pristine source for future updates. Module language
versions and historical upstream provenance do not change the Go 1.27+ support
floor. Maintain tooling in their source modules and regenerate/check vendor trees.
