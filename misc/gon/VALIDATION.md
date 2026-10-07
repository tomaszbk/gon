# Current Gon validation

Target: **Gon 2.27**, compatible with Go 1.27+ on the supported architectures,
subject to the accepted newline-after-prefix-`!` exception. The unmodified
baseline is `/opt/homebrew/bin/go`, **Go 1.27.1**. Native host execution is
**darwin/arm64**, Apple M4. The fork's Go 1.28 development version identifies
source provenance; it is not the released compatibility baseline.

The published `3356d44968` commit did not have a complete validation run on one
unchanged final source tree. The sixteen recorded profiles passed, but twelve
finished before subsequent gonpls, public ExprString and runner edits. The
bootstrap, wasm and Linux snapshots also preceded those final edits. The
historical results in the recorded sections below describe those snapshots,
not validation of the published commit or of later review fixes. Additional
platform and Docker testing remains deferred by the user.

## Cleanup of prior Gon designs

Gon's compatibility contract applies to Go 1.27+; earlier Gon designs have no
compatibility protocol. The cleanup replaced obsolete optional fixtures with
native `T?` syntax, updated refactoring diagnostics and comments, renamed
`optionresult` sources to `optionalexpr` and executable fixtures to `optionals`,
and removed prototype baseline-environment aliases. Harnesses now use only
`GON_BASELINE_GO`. Rejection tests and ordinary user-defined homonyms remain.

Seven selected optional checks passed on **darwin/arm64**, with unmodified
**Go 1.27.1**. They executed the Go/Gon pairs and rejection cases, cgo/coverage,
vet, semantic explanations, syntax and both checkers including `TestGenerate`.
Their unchanged implementation fingerprint was:

`9de8bda91bcfc7600339ab78dba2e65464a92929c0157cf1397af94ba16ed633`

The [summary](validation/20261007-cleanup/option-summary.json),
[source manifest](validation/20261007-cleanup/source-manifest.json.gz) and
compressed per-check logs in the same directory retain the exact scope. This
was a selected run; exit 2 records `--only`, with every selected check passing.
The earlier complete-profile, bootstrap, platform and benchmark records retain
their recorded source snapshots.

From the repository root, these commands passed with
`GON_BASELINE_GO=/opt/homebrew/bin/go`, `GON_ROOT=/Users/tzbk/Documents/gon`
and `GOWORK=off`:

```sh
python3 misc/gon/vendor.py --check
python3 misc/gon/build.py
python3 misc/gon/test_validate.py
python3 misc/gon/validate.py option --only syntax --only types --only optionals-execution --only optionsyntax-execution --only cgo-cover --only vet --only explain
python3 misc/gon/test.py
python3 misc/gon/test_cli.py
python3 misc/gon/test_analysis.py
./gon/bin/gon test cmd/internal/testdir -run '^Test/(errorhandling|conditional)\.go$' -count=1
./gon/bin/gon test cmd/cgo/internal/testerrorhandling cmd/cover cmd/vet -run '^(TestPairedCgoErrorHandling|TestErrorFlowCoverage|TestCondExpr)$' -count=1
```

The environment-harness checks had zero failures or skips and executed the
unmodified baseline, including coverage in count/set/atomic modes. Their logs
are retained as `env-testdir.jsonl.gz` and `env-integrations.jsonl.gz` in the
cleanup evidence directory. The validation runner's four unit tests passed.

From maintained `tools/x-tools`, with the same environment and the absolute
project tool path, this focused command passed in all three packages:

```sh
/Users/tzbk/Documents/gon/gon/bin/gon test ./internal/typesinternal ./internal/refactor/inline ./go/ssa -run '^(TestGonNoEffects|TestGonCalleeSafety|TestGonCallerSafety|TestGonCallerExecution|TestGonOptionalCalleeWithoutMetadata|TestGonFeatures|TestGonAlternatives|TestGonSyntaxSimplification)$' -count=1
```

From `tools/staticcheck`, the corresponding AST/IR checks passed:

```sh
/Users/tzbk/Documents/gon/gon/bin/gon test ./go/ast/astutil ./go/ir -run '^(TestGonTransformSafety|TestGonFeatures|TestGonAlternatives|TestGonSyntaxSimplification)$' -count=1
```

From `tools/gonpls`, `gon test ./internal/golang -run
'^TestGonFeatureExtraction$' -count=1` passed. The public tools were rebuilt
and vendor matched the maintained sources. The VS Code extension also removed
the prototype environment alias; its TypeScript check and complete macOS
integration suite passed, including Go/Gon pairs, LSP, debugger and test explorer.
Its commands and evidence are recorded in the extension repository's
`VALIDATION.md`. Additional platform and Docker runs remain deferred.

## Review corrections: one unchanged local source tree

The complete review gate passed all sixteen profiles on **darwin/arm64**,
Apple M4, with unmodified **Go 1.27.1** as the legacy baseline. Every profile
ran without `--only`, with no pending integration gates, and retained the same
implementation fingerprint before and after each check:

`24aed67972198004e3b6f8fa35692e519e198c7e6d26cd372bc88bb3d54c2025`

This identifies the local implementation, fixtures and API inventories,
including the then-uncommitted changes. The review's recorded Git HEAD is
`3356d44968dc7444011fffcef5d04f3431c0764a`; the review evidence validates the
local source fingerprint, not that published commit. Documentation, retained
evidence and ignored build output are excluded from the implementation
fingerprint. The [source manifest](validation/20261007-review/source-manifest.json.gz)
records the exact files and their hashes.

The review regressions cover contextual `is` in ordinary Go generic
constraints, named boolean pattern tests and guards, generic optional link
identities and DWARF, interpolation formatting/evaluation/diagnostics,
parenthesized record patterns, cgo errno propagation, safe modernization and
extraction, providers of `gon/seq` in modules, workspaces and vendor directories,
public API parameter names and Staticcheck unused analysis. The seq gate builds its
test binary with `gon test -c` and executes that binary; compile-only commands
are recorded without requiring test-event output.

NilAway regressions cover safe and positive ordinary Go flow in Gon packages,
method expressions, closures, dependency diagnostics, exact source positions
and reuse of serialized source facts across working directories. Diagnostic
ordering compares absolute and relative paths by the same source identity.
The upstream module suite and maintained Gon fixtures passed; internal errors
remain visible.

Complete commands used:

```sh
GON_BASELINE_GO=/opt/homebrew/bin/go GOMAXPROCS=2 \
  python3 misc/gon/validate.py PROFILE
```

| Profile | Checks | Result |
| --- | ---: | --- |
| tooling | 71 | PASS |
| errorhandling | 34 | PASS |
| conditional | 34 | PASS |
| lambda | 37 | PASS |
| nullsafety | 37 | PASS |
| namedarguments | 34 | PASS |
| enums | 40 | PASS |
| matching | 38 | PASS |
| option | 47 | PASS |
| seq | 7 | PASS |
| errorcontext | 9 | PASS |
| matchalternatives | 7 | PASS |
| patterntest | 7 | PASS |
| interpolation | 11 | PASS |
| nilanalysis | 5 | PASS |
| modern | 112 | PASS |

Counts include shared gates. [Profile evidence](validation/20261007-review/profiles.json)
links each complete summary, retained compressed logs and recorded upstream
skips. Per-check summaries record the private/public tool hashes, baseline,
platform, source fingerprint and Git HEAD separately. Executable feature pairs
were not skipped. The API parameter-name inventory is recorded only for
darwin/arm64; other platform inventories remain unrecorded.

An isolated source-only snapshot with this same fingerprint bootstrapped from
unmodified Go 1.27.1 using a fresh build cache. **All seventeen checks passed**:
`make.bash` and sixteen executable feature harnesses.
[Bootstrap evidence](validation/20261007-review/bootstrap.json) retains commands,
logs, compiler hashes and the source manifest reference; the temporary snapshot
was removed.

These review runs executed only on macOS arm64. Earlier benchmark, Linux, wasm
and PostgreSQL results below retain their original snapshot scope. No new
non-macOS, PostgreSQL or Docker validation was started.

## Scope and regression evidence

The eight original features and accepted additions share both parsers/checkers,
public AST/formatting, export data, native compiler lowering, maintained tooling
and executable compatibility pairs. Error handling uses ordinary Go tuples
ending in error. The predeclared Result, contextual constructors, tuple bridges,
ErrNilResult and optional migration command are removed. User-declared homonyms
and ordinary Go error/nullable semantics remain valid.

Paired harnesses execute the legacy program with unmodified Go 1.27.1 and Gon,
and the equivalent modern program with Gon. They assert output, effects,
success/failure/absence and relevant boundaries; selected harnesses repeat with
`-l` and `-N -l`. Compile rejection and cross-package/export cases are included.
The focused checks are distinct from the final complete profile gates.

Focused checks passed for multi-pattern arms, pattern tests with bindings,
one-line error context, interpolation and the 18 functions in `gon/seq`.
The interpolation review adds raw-text preservation, nested match/function/
handler expressions, format/comment handling and physical-file import checks.
The native pattern-test lowering places condition bindings in the enclosing
if initialization; regressions cover multi-name bindings and optional record
patterns. cgo/coverage pairs cover matching, native optionals, error context
and interpolation. Structural tools, SSA interpretation, Staticcheck IR,
analyzer diagnostics, semantic CLI, LSP and conservative refactor behavior
have focused regressions.

## Maintained NilAway

`tools/nilaway` preserves Uber's Apache-2.0 license, NOTICE, upstream corpus and
module path. [Provenance](../../tools/nilaway/UPSTREAM.json) records upstream
revision `acb8859b9031bb9496be97e027df5573f9fb5340`.
Ordinary Go uses its upstream analysis; Gon packages feed typed maintained SSA
into NilAway's inference triggers. The review found false positives in safe Go
functions merely sharing a package with a Gon construct, and an analysis failure
on method expressions. Earlier no-panic fixtures did not establish precision.
Named calls use the associated parameters.
Gon safe/unsafe fixtures and executable pairs cover the language constructs,
alias/closure writes, container and deep-field flow, generic optional payloads,
interfaces, receiver flow and Fatal in tests. NilAway stays opt-in through
`gon check --nilaway` and `"gon.serverSettings": { "nilaway": true }`; internal errors
are visible and never treated as successful nil analysis.

## PostgreSQL

```sh
GON_BASELINE_GO=/opt/homebrew/bin/go GON_SQL_POSTGRES=1 \
  python3 misc/gon/test_optionals_postgres.py
```

PASS against disposable **PostgreSQL 17** with **pgx stdlib** and **lib/pq**.
The legacy baseline, Gon legacy, Gon modern and non-inlined modern execute for
both drivers. Optional NULL/presence, JSON composites, transactional decoding,
strict struct scanning and ordinary Scan conversions are asserted.
The server uses temporary container storage and is removed after the test.
[Versions, commands and results](benchmarks/results/20261007T033706Z/validation/postgres.json).
Native pgx remains an explicit codec boundary.

## Recorded bootstrap snapshot

A source-only isolated snapshot bootstrapped from unmodified Go 1.27.1 with a
fresh build cache. It excludes repository Git metadata, built tools, ignored
working documents and old benchmark output.

```sh
# In the snapshot's src, with a fresh temporary GOCACHE:
env -u GOROOT -u GOTOOLDIR \
  GOROOT_BOOTSTRAP=/opt/homebrew/Cellar/go/1.27.1/libexec \
  GOMAXPROCS=2 GOFLAGS=-p=2 ./make.bash
```

PASS: all bootstrap stages and command-staleness checks, legacy smoke against
the baseline, native optional core/boundary harnesses and every new compiler/
library feature pair. [Snapshot evidence](benchmarks/results/20261007T033706Z/validation/bootstrap.json).
The deterministic algorithm-bearing type ordering fix remains covered by
`TestTypesStrCmpNoalg`; a fresh cache is not a substitute for that correction.

## Recorded profiles and platforms

The complete js/wasm pair script passed with Node 26.6.0 and GOMAXPROCS=1.
The wasm runtime requires a single P. This run includes legacy baseline, Gon
legacy, modern, non-inlined modern and Fatal tests; fixture hashes stayed
unchanged during execution. [Wasm result](benchmarks/results/20261007T033706Z/validation/wasm.json).
Linux execution that was already active completed before the user deferred
additional platform/Docker tests to the following day. It passed all target
pairs on linux/arm64 natively in the Docker Linux VM and linux/amd64 and
linux/riscv64 through QEMU user-mode. The validation containers were removed.
[Linux commands and target results](benchmarks/results/20261007T033706Z/validation/linux.json).
No further platform or Docker test is scheduled. These macOS profiles passed
across successive source states; they are not a single final-snapshot gate.
Profiles are focused on the compiler/tooling packages and feature harnesses;
no whole standard-library, cmd, dist or complete test-directory run is used.

Complete commands used the unmodified baseline:

```sh
GON_BASELINE_GO=/opt/homebrew/bin/go GOMAXPROCS=2 \
  python3 misc/gon/validate.py PROFILE
```

Each PROFILE below ran without `--only`. Check counts include shared gates;
they are not counts of distinct tests across profiles.

| Profile | Checks | Result |
| --- | ---: | --- |
| tooling | 63 | PASS |
| errorhandling | 26 | PASS |
| conditional | 26 | PASS |
| lambda | 29 | PASS |
| nullsafety | 29 | PASS |
| namedarguments | 26 | PASS |
| enums | 32 | PASS |
| matching | 30 | PASS |
| option | 39 | PASS |
| seq | 4 | PASS |
| errorcontext | 9 | PASS |
| matchalternatives | 7 | PASS |
| patterntest | 7 | PASS |
| interpolation | 10 | PASS |
| nilanalysis | 5 | PASS |
| modern | 100 | PASS |

[Commands, results and recorded skips](benchmarks/results/20261007T033706Z/validation/profiles.json).
Per-profile summaries link retained compressed logs. Upstream manual
`TestErrorCodes` and the vet fixtures intentionally skipped by upstream are
recorded in those summaries; executable feature pairs were not skipped.
The recorded vendor consistency check passed. Whitespace review found only
retained upstream NilAway documentation/Makefile whitespace; Gon adaptations
have no new whitespace errors.

## Benchmarks

The current fixtures contain **22 workloads** and **16 correctness tests** per
variant, covering the eight original features and runtime additions. Correctness
passes for baseline legacy, Gon legacy and Gon modern with identical output.
The recorded default measurement run passed: ten runtime rounds at 300 ms and
seven build samples, with compiler and fixture hashes unchanged during the run.
[Complete measured report](benchmarks/results/20261007T033706Z/report.md) and
[raw data](benchmarks/results/20261007T033706Z/summary.json) retain all 22 workloads,
including regressions, build costs, allocation counts and representation sizes.
All 22 workloads have identical heap bytes and allocation counts across the
three variants. Native unit enums occupy 16 bytes versus 8 for the tagged Go
fixture; `struct{}?` occupies 16 versus 2. Pointer and integer optionals occupy
16 bytes in all variants. Main-package rebuild medians are 0.249 seconds for
Gon legacy and 0.251 for Gon modern. Measurements apply to this host/session,
with no stable ABI or zero-overhead claim. See [method and workloads](benchmarks/README.md).

## Product migration

`/Users/tzbk/Documents/llm_teacher` uses tuples, multi-pattern arms, `is`,
one-line error context, raw interpolation, seq.Lookup/Map, a typed generic JSON
handler and cmp.Or. Pre-existing user changes are preserved.

### Recorded migration snapshot

Its recorded backend unit suite, vet, standard `gon check` (164 analyzers, zero diagnostics)
and PostgreSQL integration with `-race` passed. The recorded NilAway opt-in check
ran 165 analyzers: zero errors, **16 warnings**, zero internal analysis failures.
Those warnings cannot be attributed to conservative framework limits: the
review reproduced defects in ordinary field, range, channel, receiver, append
and type-switch flow in packages using Gon. [Standard check](benchmarks/results/20261007T033706Z/validation/llm-teacher-check.json),
[opt-in check](benchmarks/results/20261007T033706Z/validation/llm-teacher-nilaway.json).
The backend Dockerfile pins the published Gon commit. No additional Docker
build or execution was requested. The prompt has an exact-output equivalence
regression.

### Review checks

The review reran semantic checks with the rebuilt tools: standard checking
used **164 analyzers with zero diagnostics**; opt-in NilAway used **165
analyzers with eight warnings, zero errors and zero internal errors**. Both
commands returned 0, with empty stderr and no terminal styling.
[Current standard result](validation/20261007-review/llm_teacher-check-standard.json),
[current NilAway result](validation/20261007-review/llm_teacher-check-nilaway.json).

The [warning review](validation/20261007-review/llm_teacher-warning-review.md)
examines all eight diagnostics: three lose the embedded filesystem's non-nil
element guarantee, two lose HTTP success/error correlation, two concern the
HTTP handler's Request.URL precondition, and one loses MCP SDK nil-parameter
normalization. No application nil defect was established by those diagnostics.
This review does not guarantee absence of nil panics; external contracts and
upstream field inference without object sensitivity remain limits.

Backend `gon test ./...` and `gon vet ./...` passed earlier in this task.
Their original recording time is preserved as `testAndVetEvidenceRecordedAt`
in the [product summary](validation/20261007-review/llm_teacher-validation-summary.json),
which links the retained logs. Those suites were not rerun after the final
runner and analyzer-position corrections. The latest standard/NilAway checks
and cross-directory capture probes are recorded separately. Application
source and its pre-existing changes were preserved.

## Editor

The macOS VS Code 1.140.0 language/configuration host, actual tokenizer,
TypeScript build, executable pairs and packaged VSIX checks passed. The
`gon.serverSettings` object forwards `nilaway` and Check Project adds the flag
only when enabled. Retired snippets are removed and nested quoted/raw
interpolation is highlighted. [Commands and scope](benchmarks/results/20261007T033706Z/validation/vscode-gon.md).
The separate debugger/test-explorer suite was not rerun in this language change.

## Supported limits

Native enum/optional storage has no stable ABI. C boundaries, XML, gob and
native pgx need explicit adapters. Native optional JSON/SQL rejects nested
layers, optional JSON keys and RawBytes payloads. Interface matching requires
a default/wildcard and does not accept type-parameter interface subjects.
Source inlining/extraction conservatively decline unsupported lazy, named or
contextual moves. NilAway warnings remain optional. Platforms not explicitly
listed as executed are not covered by this execution evidence.
The API parameter-name inventory is currently recorded only for darwin/arm64.
NilAway retains limits for unknown external contracts and upstream field
inference without object sensitivity; a clean check does not guarantee absence
of nil panics.
