# Current Gon status

Gon 2.27 is the development target, compatible with Go 1.27+ on amd64, arm64,
riscv64 and wasm subject to the accepted newline-after-prefix-`!` exception.
Unmodified Go 1.27.1 is the baseline; the fork's Go 1.28 development version
identifies provenance. Keep `.go`, `_test.go`, `go.mod` and `go.work` and select
public `gon`/`gonpls` per project alongside ordinary Go.
Compatibility guarantees cover Go 1.27+ source. Earlier Gon designs and syntax
do not have a compatibility protocol.

The eight original features are error propagation, native optional values,
exhaustive pattern matching, enums, lambdas, nil-safety operators, conditional
expressions and named arguments. The accepted additions are several patterns
per match arm, `is` pattern tests, one-line error context, string interpolation,
`gon/seq` and opt-in NilAway. The native compiler, both frontends/checkers,
public AST/formatting, exports, cgo/coverage, CFG/SSA, Staticcheck IR and gonpls
share their contracts. [features.json](features.json) records open integration
checks and supported limits; [VALIDATION.md](VALIDATION.md) records actual runs.
A local pass does not establish execution on other targets.

The review closure passed all sixteen complete profiles and a source-only
Go 1.27.1 bootstrap on one unchanged local implementation fingerprint on
darwin/arm64. The retained source manifest and per-check tool hashes identify
that local tree independently of the published Git HEAD; other platform
evidence remains historical and additional platform/Docker runs remain deferred.

Subsequent cleanup removed obsolete design names and test-environment aliases.
Its affected checks passed on darwin/arm64 with Go 1.27.1; the earlier complete
review gate retains its own source fingerprint. See the cleanup record in
[VALIDATION.md](VALIDATION.md#cleanup-of-prior-gon-designs).

The current upstream integration incorporates 119 Go master commits through
`a99091efdb38ba38e85970b726fb92c1882f8d97`. All six complete native darwin/arm64
profiles passed: tooling, modern, errorhandling, conditional, lambda and
nullsafety. A fresh source-only Go 1.27.1 bootstrap and all sixteen executable
harnesses also passed; the compiler source remains identical to that snapshot.
The recorded inventories distinguish the later tooling-fixture correction and
parallel Sponsors metadata from compiler changes. See the commands, exact
source scopes and retained attempts in
[VALIDATION.md](VALIDATION.md#upstream-integration). Additional platform,
PostgreSQL and Docker runs remain deferred, and timed benchmark evidence keeps
its historical snapshot scope.

API checks include native optional signatures and a separate inventory of
public parameter names, including nested signatures and reachable private
types. The name inventory is recorded only for darwin/arm64; other platform
inventories remain unrecorded while additional platform validation is deferred.

Error handling uses ordinary Go tuples ending in `error`. `!` and
`or err => expression` propagate errors and zero the other results before
defers; a block `or` handler chooses its own behavior. Both propagation forms
require the nearest function to return `error` last, also in `_test.go` files.
Tests report failures explicitly with block handlers and `Fatal`; the operators
have no testing-specific behavior. Existing tuples can still carry useful partial results for
explicit handling. User-declared names retain ordinary Go semantics.

Native `T?` uses untyped nil for absence and lifts one immediate payload for
presence, including typed nil and zero. Nested layers never flatten. Match
presence with `P?` and absence with nil. Optional equality against untyped nil
checks presence even for noncomparable payloads. Hover explains that presence
does not imply a non-nil payload. See [OPTIONALS.md](OPTIONALS.md).

`encoding/json` (v1/v2) and `database/sql` map optional absence to JSON null and
SQL NULL. Failed decoding/scanning preserves the destination. Nested optionals,
optional JSON map keys and SQL RawBytes payloads are rejected. JSON columns
can decode into optional composites and ScanStruct fields; PostgreSQL array
text requires a codec or JSON projection. Real PostgreSQL pairs run against
pgx/stdlib and lib/pq without native pgx codecs.

`Rows.ScanStruct`, `Row.ScanStruct`, `Collect[T]` and `CollectOne[T]` use strict
column-to-exported-field mapping, with exact sql tags or lowercased field names.
Unmapped, ambiguous and duplicate columns are errors. Ordinary Scan conversion
rules, custom Scanners, optionals and string enums retain precedence.

String enums opt in through `type Role enum string`, a default string payload
and unit variants with unique spellings. Parse preserves unknown text; generated
String/MarshalText/pointer UnmarshalText support standard JSON and database/sql.
JSON null preserves existing enum values; SQL NULL uses pointers, Null[T] or
native optionals. Ordinary enums and native pgx need explicit adapters.

Enum patterns on interface subjects require the enum value to implement the
interface. Error subjects search the error tree with errors.AsType semantics;
other interfaces use an exact assertion. A nil interface matches no enum arm,
and default or wildcard is required. Type-parameter interface subjects are not
supported.

The unreachable analyzer inspects block `or` handlers. Inline variable formats
handler initializers correctly, and type queries distinguish ordinary error
propagation and one-line context. Source inlining/extraction decline
unsupported movement across named arguments or lazy/function boundaries.

NilAway is maintained at `tools/nilaway`, with upstream Apache-2.0 license,
NOTICE and provenance. Enable it with `gon check --nilaway` or editor
`"gon.serverSettings": { "nilaway": true }`. Warnings are opt-in and ordinary Go nil
semantics are unchanged. Upstream corpus and Gon safe/unsafe pairs validate its
adaptation; internal analysis errors remain visible. Safe Go-only functions in
packages with Gon syntax and corresponding positive cases have precision
regression coverage. Upstream field-assignment inference still has object-sensitivity
limits: an imported constructor with an implicit nil field can lack a warning.
A clean result does not establish absence of nil panics.

Representation uses a discriminator and separate typed GC-safe storage. Enums
whose variants all have no payload fields use the smallest sufficient tag:
up to 256 variants occupy one byte, without trailing empty-field padding.
Declared zero-size payloads and optionals retain their existing layouts. No
stable ABI or zero-overhead claim is made. Reflection exposes checked package
functions and detached payloads. C boundaries and serializers beyond the
supported JSON/SQL interfaces need explicit adapters. Positional constructor
functions and generated string-enum methods currently disable compiler inlining.
See [benchmarks/README.md](benchmarks/README.md) for measured costs.

Maintained modules are `tools/x-tools`, `tools/staticcheck`, `tools/gonpls` and
`tools/nilaway`; vendor trees are generated. [INTEGRATION.md](INTEGRATION.md)
defines the per-feature gate. [UPSTREAM.json](UPSTREAM.json) identifies integrated
Go revision a99091efdb38ba38e85970b726fb92c1882f8d97 for future three-way updates.
Preserve licenses, module paths, upstream tests and provenance.

Root AGENTS.md and design Markdown remain local and ignored. Gon 2.28 would
adopt Go 1.28; compatibility-breaking work is paused and does not constrain 2.27.
