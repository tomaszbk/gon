# Current Gon status

Gon 2.27 is the current development target, compatible with Go 1.27+ on the
supported platforms subject to accepted exceptions. Go 1.27.1 is the unmodified
baseline; the fork's go1.28-devel version identifies provenance. Keep `.go`,
`_test.go`, `go.mod` and `go.work`. Select public `gon`/`gonpls` per project,
alongside ordinary Go. See the [adoption guide](../../README.md#use-gon-in-an-existing-go-project).

All nine native cores and maintained tooling are implemented: error propagation,
conditional expressions, lambdas, nil safety, enums, exhaustive matching,
optional values, Result and named arguments. Both frontends/checkers, public
AST/formatting, lowering, metadata readers, cgo/coverage, CFG/SSA, Staticcheck IR
and gonpls are integrated. Maintained modules are `tools/x-tools`,
`tools/staticcheck` and `tools/gonpls`; vendor trees are generated.

Optional values now use the native type `T?`, nil absence, immediate payload
lifting and presence patterns `P?`. Typed nil remains present; nested layers
never flatten. No predeclared Option/Some/None API remains. Result retains
`.Ok/.Err` and qualified constructors/patterns. Postfix `!` bridges Go error
tuples and Result in both directions; other conversions stay explicit.
The [optional contract](OPTIONALS.md) explains reflection, JSON/`database/sql`
and the reviewable `gon refactor optionals` migration. Ordinary user names
retain Go semantics.

[VALIDATION.md](VALIDATION.md) records the latest commands, complete focused
profile results, bootstrap, editor and target evidence. Executed targets and
cross-compiled targets are distinguished. [features.json](features.json)
records supported limits; [INTEGRATION.md](INTEGRATION.md) defines the maintained
workflow. These checks do not establish a complete release pass on every
supported platform.

String enums opt in with `type Role enum string`, a default string payload and
unit variants with unique constant spellings. Native `Role.Parse`, `String`,
`MarshalText` and pointer `UnmarshalText` remove per-enum text/JSON adapters.
Unknown text is preserved and JSON null preserves an existing value. Both
frontends/checkers, imports, reflection metadata, SSA/IR, cgo/coverage and editor
services retain the protocol. See [the contract](README.md) and the executable
`test/stringenums.go` pair. Gon's `database/sql` maps string enums directly to
text parameters and parses text/byte results, including direct-column scanners.
Use `sql.Null[Role]`, `*Role` or `Role?` for SQL NULL; explicit Scanner/Valuer
overrides retain precedence. No enum registration or driver changes are needed
through database/sql. Native pgx (outside `database/sql`) still needs codecs.

Generated `UnmarshalText` allocates no memory for known variants; only the
unknown-text fallback copies its input into a retained payload. SQL JSON
columns decode into native optional composite payloads and `ScanStruct`
composite fields without per-domain Scanners. JSON null is optional absence,
and errors preserve the destination. Explicit Scanners and existing scalar,
cursor and decimal conversions retain precedence. Raw PostgreSQL arrays need a
codec or a JSON query projection. Ordinary Go `Rows.Scan` conversions remain
unchanged. See the paired `test/sqljson.go` regression.

Interoperability extensions requested and accepted on 2026-10-05 (user decision,
part of Gon 2.27), merged on master `96ab86645a`:

- **Optional JSON and `database/sql`.** `encoding/json` (v1 and the default
  v2-backed implementation) encodes absence as `null` and presence as the
  payload; `null` decodes to absence, and a failed decode leaves the optional
  unchanged. `database/sql` binds an optional argument as NULL or its payload and
  scans NULL into absence, otherwise through the ordinary conversion of the
  payload (string enums, uuid, Scanners). `reflect.OptionalValueSetPayload` is
  the new checked reflection setter. `encoding/xml`, `gob` and native pgx are not
  covered. See [OPTIONALS.md](OPTIONALS.md#json-and-sql).
- **`database/sql` struct scanning.** `Rows.ScanStruct`, `Row.ScanStruct`,
  `Collect[T]`, `CollectOne[T]` and `ErrTooManyRows` map columns to exported
  fields strictly (exact `sql:"name"` tag, else name with case and underscores
  ignored). See [README.md](README.md#sql-struct-scanning).
- **`!` in test functions and across tuples/Result.** In `_test.go` files `!`
  reports with `Fatal`; a Go error tuple fails as `Result.Err` in a Result
  function; a failed Result fails as its payload in an error-returning function,
  with `Err(nil)` becoming `errors.ErrNilResult`. See [README.md](README.md#error-propagation-across-tests-tuples-and-result).
- **Enum patterns on interface and error subjects.** `=>` matches accept
  qualified enum variant patterns on interface subjects; on `error` each arm
  searches the error tree with `errors.AsType` semantics. See
  [README.md](README.md#matching-interface-and-error-subjects).
- **Fixes.** Untyped comparisons passed as named arguments no longer crash the
  compiler; vet `httpresponse` and `sqlrowserr` understand `!`/`or` handlers;
  gonpls no longer crashes on methods returning the predeclared `Result`.

Fixed after the feature merges (branch `gon/fixes`, `2b0ee32fbc`):

- Optional types (`T?`, `*T?`, `[]T?`, `(T?)?`, ...) compile in type-switch
  cases, assertions, literals, conversions, instantiation and anonymous-interface
  cycle detection; covered by `test/optionsyntax.dir/typeforms_*.go` and
  per-form compile checks in `test/optionsyntax.go`.
- objectpath encodes enum variant payload fields directly, so variants with two
  or more positional payloads no longer panic `gon check`
  (`TestEnumPathsPositionalPayloads`).
- `reflect` no longer imports `strings` (enum metadata is split with
  `internal/stringslite`), restoring `go/build` `TestDependencies`.
- String-enum method synthesis no longer forces `Named.Underlying` of
  in-progress declarations, fixing the `TestCheck/cycles0.go` panic and a crash on
  valid cycles such as `type A B; type B *A` (`TestStringEnumDeclarationCycles`).
- The copylock `gonfeatures` expectation sits on the reported `return` line.

Optional comparisons with untyped nil (`x == nil`, `x != nil`, in either
operand order) test absence/presence without comparing the payload. Noncomparable
payloads work; a present typed nil remains present. Typed or shadowed nil values
retain ordinary type rules.

Known tooling gaps: the `unreachable` analyzer does not inspect `or` handler
bodies; inline variable on an `or`-handler initializer yields an oddly
formatted edit; `gon query refs Result` reports builtin references as
unsupported; `gon query type` does not name the new `!` sub-kinds; test, tuple
and Result `!` have only gonerrors suggestions, no dedicated editor action.

Representation uses a discriminator and separate typed storage for GC safety.
Unit and zero-sized shapes have measured overhead. There is no stable storage
ABI. Ordinary enums and C boundaries need explicit adapters, as do serializers
other than `encoding/json` and `database/sql` for optionals; string enums supply
the standard text interfaces. First-class positional constructors,
string parsers and generated string enum methods disable compiler inlining.
Source inlining/extraction conservatively decline unsupported named, lazy and
contextual moves. The [current benchmark](benchmarks/README.md) publishes all
24 workloads and their costs. No zero-overhead claim is made.

[UPSTREAM.json](UPSTREAM.json) records integrated Go revision
`67c1d421161d3d1ae9f5fd005e84c29fd0d9f896`. Integration used three-way source
comparison without importing upstream merge ancestry. Use that revision as the
previous pristine source for updates; preserve each tooling module's provenance,
upstream licenses, paths and tests.

Update current records in place. Handovers and superseded published experiments
are removed. Root AGENTS.md and design Markdown remain local and ignored.
Legacy Go fixtures remain required compatibility tests. Gon 2.28 would adopt
Go 1.28; Gon 3.0 remains a future, separately specified language direction.
