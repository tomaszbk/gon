# Current Gon validation

Target: **Gon 2.27**, compatible with Go 1.27+ on the supported architectures,
subject to the accepted newline-after-prefix-`!` exception. The unmodified
baseline is `/opt/homebrew/bin/go`, **Go 1.27.1**. Native host execution is
**darwin/arm64**, Apple M4. The fork's Go 1.28 development version identifies
source provenance; it is not the released compatibility baseline.

This record replaces obsolete evidence for retired language constructs.
All sixteen complete validation profiles passed on darwin/arm64 after the
source review, with no pending integration items. Bootstrap, paired correctness,
benchmarks and the product/editor checks below passed. Additional platform and
Docker testing is deferred by the user until the following day.

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
into NilAway's inference triggers. Named calls use the associated parameters.
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

## Bootstrap

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

## Final profiles and platforms

The complete js/wasm pair script passed with Node 26.6.0 and GOMAXPROCS=1.
The wasm runtime requires a single P. This run includes legacy baseline, Gon
legacy, modern, non-inlined modern and Fatal tests; fixture hashes stayed
unchanged during execution. [Wasm result](benchmarks/results/20261007T033706Z/validation/wasm.json).
Linux execution that was already active completed before the user deferred
additional platform/Docker tests to the following day. It passed all target
pairs on linux/arm64 natively in the Docker Linux VM and linux/amd64 and
linux/riscv64 through QEMU user-mode. The validation containers were removed.
[Linux commands and target results](benchmarks/results/20261007T033706Z/validation/linux.json).
No further platform or Docker test is scheduled. The final macOS profiles all passed.
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
The final vendor consistency check passed. Whitespace review found only
retained upstream NilAway documentation/Makefile whitespace; Gon adaptations
have no new whitespace errors.

## Benchmarks

The current fixtures contain **22 workloads** and **16 correctness tests** per
variant, covering the eight original features and runtime additions. Correctness
passes for baseline legacy, Gon legacy and Gon modern with identical output.
The final default measurement run passed: ten runtime rounds at 300 ms and
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
Its backend unit suite, vet, standard `gon check` (164 analyzers, zero diagnostics)
and PostgreSQL integration with `-race` passed. The final NilAway opt-in check
ran 165 analyzers: zero errors, **16 conservative warnings**, zero internal
analysis failures. Twelve concern external constructor/framework invariants
(embed directory entries, MCP tools, Request.URL and injected services); four
concern HTTP response/body return facts. The source guard, enum payload and
Fatal flow regressions are fixed. [Standard check](benchmarks/results/20261007T033706Z/validation/llm-teacher-check.json),
[opt-in check](benchmarks/results/20261007T033706Z/validation/llm-teacher-nilaway.json).
The backend Docker commit pin advances to the published Gon work branch commit
as the final migration step; no additional Docker build or execution was
requested. The prompt has an exact-output equivalence regression.

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
