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
live integration separately. pgx's native API and native `Role?` SQL mapping
remain separate adapter boundaries.

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
| darwin/arm64 | Executed | Focused gates, baseline, bootstrap, benchmark, real LSP and editor |
| js/wasm | Executed through Node 26.6.0 | Eleven portable pairs, including string enums, SQL, analysis fixtures and local generic cases; 11 baseline and 33 Gon legacy/modern/non-inlined executions match |
| linux/amd64 | Cross-compiled | String-enum common/analysis and SQL pairs: baseline legacy, Gon legacy, Gon modern; existing optional/Result evidence retained below |
| linux/riscv64 | Cross-compiled | Same three variants and all three string-enum pairs |

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
  and standard JSON/`database/sql` conversions. SQL NULL uses `sql.Null[T]` or
  pointers. Native pgx and native optional SQL mapping need explicit adapters.
- Native optional lifting is single-valued and one layer. Typed nil is present;
  nested layers and mixed optional/Go-nil guards require explicit boundaries.
- Result constructors need known targets. Go tuple boundaries need explicit
  adapters because useful partial results may coexist with errors.

## Upstream integration

[UPSTREAM.json](UPSTREAM.json) records integrated Go source revision
`67c1d421161d3d1ae9f5fd005e84c29fd0d9f896`. A three-way source comparison carried
Gon adaptations forward; upstream merge ancestry was not imported. Use that
revision as the previous pristine source for future updates. Module language
versions and historical upstream provenance do not change the Go 1.27+ support
floor. Maintain tooling in their source modules and regenerate/check vendor trees.
