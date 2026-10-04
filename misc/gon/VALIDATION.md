# Current Gon validation

Current source target: **Gon 2.27**. Unmodified baseline:
`/opt/homebrew/bin/go`, **Go 1.27.1**. Native execution is **darwin/arm64**,
Apple M4. The fork's go1.28-devel version records source provenance. This file
records current evidence in place; no full release pass on every platform is claimed.

## Complete focused gates

```sh
GON_BASELINE_GO=/opt/homebrew/bin/go python3 misc/gon/validate.py modern
GON_BASELINE_GO=/opt/homebrew/bin/go python3 misc/gon/validate.py tooling
```

| Profile | Result | Sum of check durations | Evidence |
| --- | --- | ---: | --- |
| modern | 47/47 PASS, exit 0, no pending items | 126.339s | [summary](benchmarks/results/20261004T013816Z/diagnostics/modern/summary.json) |
| tooling | 36/36 PASS, exit 0, no pending items | 128.348s | [summary](benchmarks/results/20261004T013816Z/diagnostics/tooling/summary.json) |

Modern deduplicates enums, matching, optional values, Result and named arguments.
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

## Source-only bootstrap

A fresh source-only snapshot bootstrapped from unmodified Go 1.27.1:

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
| js/wasm | Executed through Node 26.6.0 | Portable pairs for all nine features and native optional syntax/boundaries; baseline legacy, Gon legacy/modern/non-inlined outputs match |
| linux/amd64 | Cross-compiled | Optional/Result and optional syntax pairs: baseline legacy, Gon legacy, Gon modern |
| linux/riscv64 | Cross-compiled | Same three variants and both pairs |

[Wasm output](benchmarks/results/20261004T013816Z/diagnostics/wasm.log) and
[12 cross-build commands/results](benchmarks/results/20261004T013816Z/diagnostics/cross-builds.json)
are retained. Linux target binaries were not executed locally; cross-compilation
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

The separate vscode-gon repository's full `npm test` passed with VS Code 1.139.1
and Delve 1.27.2. It tests native optional patterns and Result-only contextual
constructors, real client services, equivalent baseline/legacy/modern execution,
automatic discovery, per-folder overrides, Run/Check Project, debugger and test
explorer. Actual tokenizer and `npm run package` also passed. VSIX inspection
confirms eleven current snippets and the grammar. See that repository's
VALIDATION.md for the exact commands and platform limits.

## Supported limits

- Source inlining/extraction decline unsupported named, lazy, propagation,
  matching or target-sensitive moves. First-class enum constructors disable
  compiler inlining.
- Optional storage is GC-safe but private, with discriminator/typed payload fields.
  Unit enums occupy 16 versus a legacy tag's 8 bytes; optional empty structs
  occupy 16 versus a legacy struct's 2 on this host. There is no stable ABI or
  automatic serialization/C mapping; explicit adapters are required.
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
