# Gon commands and language server

`gon` runs this fork's Go-compatible command. `gonpls` runs a Gon-aware build of
gopls. Both pin their subprocesses to this toolchain without replacing the user's
Go installation. Source files still use `.go`.

Gon 2.27 targets backward compatibility with **Go 1.27+**, subject to the
accepted newline-after-prefix-`!` exception. The user selected this support
floor. Use stable Go 1.27.1 as the current
unmodified baseline; the fork's Go 1.28 development version is not a released
Go baseline. Module language directives retain their independent meaning.
The compatibility contract covers Go source; earlier Gon designs and syntax
do not have a compatibility protocol.

`gon fix -diff ./...` previews opportunities to adopt Gon syntax; `gon fix
./...` applies the safe conversions. This includes error propagation/handlers,
conditional expressions, nil-safety operators and contextually typed lambdas.
The same optional hints are available in the editor and with `gon check
--severity=hint ./...`. See [the modernization commands and limits](CLI.md#syntax-modernization).

Current implementation and evidence are consolidated in [STATUS.md](STATUS.md)
and [VALIDATION.md](VALIDATION.md). The [native optional contract](OPTIONALS.md)
describes presence, absence and serialization.

## Closed alternatives and named calls

Gon implements contextual `type Name enum` declarations, qualified unit,
positional and record constructors, exhaustive statement/expression matching,
native optional `T?` and named arguments.
Existing Go switches, function types and positional calls retain their semantics.

```go
type Message enum {
    default Empty
    Text(string)
    Count { Number int }
}
func describe(m Message) string {
    return switch m {
    case Message.Empty => "empty"
    case Message.Text(text) => text
    case Message.Count{Number: n} => fmt.Sprint(n)
    }
}
var selected int? = 3
value := selected ?? 0
out := combine(second: 2, first: 1)
```

`T?` is a native optional type. Assign an immediate payload for presence or
untyped `nil` for absence. A typed nil pointer remains present. The postfix
binds tightly: `*T?` is a pointer to an optional, `(*T)?` is an optional pointer,
`([]T)?` is an optional slice, and `(T?)?` contains two distinct optional layers.
Aliases preserve this protocol; separately defined types do not adopt it.
`selected == nil` and `selected != nil` test absence/presence, including for
noncomparable payloads; a present typed nil remains present.

```go
func parsePort(text string) (port int?, err error) {
    if text == "" { return nil, nil }
    value := strconv.Atoi(text)!
    return value, nil
}
var selected int? = 3
var absent int? = nil
var pointer *int
var presentPointer (*int)? = pointer // present, with a nil payload
```

There are no predeclared `Option`, `Some` or `None` constructors. Existing user
identifiers with those spellings remain ordinary Go declarations. Native
presence patterns use `case value?`; `case nil` matches absence. For optional
pointers, `case nil?` matches a present nil pointer. Nested `(value?)?` patterns
check both presence layers. An explicit `(T?)(payload)` conversion is available
when no assignment target provides the type. Each lift constructs exactly one
layer; nested values never flatten or recursively lift.

Enums require qualified variants and an explicit default variant. Record
construction uses field names and Go zeros for omitted fields; partial record
patterns require `...`. New matching is exhaustive. Payload identifiers bind
fresh arm variables; true/false/nil are contextual values even under shadowing.
Use guards to compare outer variables or local constants.

An optional's zero value is absent. `?` propagates absence to a function
returning one optional. `!` handles calls ending in `error`; `or problem { ... }`
handles them locally, and `or problem => expression` propagates a contextualized
error. Optional and legacy nil navigation require an explicit payload boundary
between chains. See [OPTIONALS.md](OPTIONALS.md).

Named calls use visible signature parameter names. Evaluate callee/receiver
first, then every argument once in written order before passing values in
parameter order. Arguments are associated before generic inference. Positional
prefixes may precede named arguments; an unnamed signature is positional-only.
Named variadics use a final compatible `name: slice...` or omit the parameter.
Untyped comparisons, `!`, `&&` and `||` results are converted to the parameter
type, so `require(valid: len(title) >= 2, ...)` needs no `bool(...)`.
There are no defaults, optional parameters or function overloading.

String enums declare their text once and retain closed, exhaustive alternatives:

```go
type Role enum string {
    default Unknown(string)
    Teacher = "teacher"
    Student = "student"
}

type loginRequest struct {
    Role Role `json:"role"`
}

role := Role.Parse("teacher")
text := role.String()
```

The default is one positional `string` payload (aliases of string are allowed);
its zero is `Unknown("")`. Other variants are units with distinct constant string
spellings, including an empty spelling if desired. `enum` and its contextual
`string` marker reserve no ordinary Go identifiers. `Role.Parse(text)` returns
a known variant when its spelling matches, otherwise `Role.Unknown(text)`.
The parser is also a function value and accepts `text:` named arguments.
An explicitly constructed `Role.Unknown("teacher")` retains its alternative
until parsed or decoded from text; wire text carries the spelling, not variant
identity.

The compiler supplies `String() string`, `MarshalText() ([]byte, error)` and
`(*Role).UnmarshalText([]byte) error`. These are ordinary methods for interfaces,
method expressions, aliases, generics and imported packages. Their names and
`Parse` cannot be redeclared on a string enum. Text marshaling copies the bytes;
unmarshaling recognized variants allocates no memory, while unknown text is
copied into its retained payload. Both paths replace the whole value and clear
inactive payload storage. Standard JSON uses
these text interfaces for values and map keys, preserving unknown strings.
JSON `null` leaves an existing value unchanged; non-string input fails without
changing it. Ordinary JSON options, escaping and UTF-8 replacement still apply.
The type retains enum storage and zero semantics: struct-related tags such as
`omitempty` or `,string` do not acquire the rules of an underlying Go string.
Gon's `database/sql` also accepts string enums directly: pass `role` to `Exec`
or `Query` and scan into `&role`. It maps arguments to strings before driver
checking, including drivers that bypass the default parameter converter. Text
and byte-slice results use the enum's parser; other source types and SQL `NULL`
fail without changing the enum. Unknown text is preserved. Aliases, generics,
prepared statements, transactions and named SQL parameters use the same path.
Use `sql.Null[Role]`, `*Role` or a native `Role?` for nullable columns (see
[OPTIONALS.md](OPTIONALS.md#json-and-sql)). The zero `Unknown("")` is present
empty text, distinct from SQL `NULL`. Explicit `sql.Scanner` and `driver.Valuer`
methods retain precedence. Drivers with direct column scanning receive an
internal Scanner adapter automatically.

This support is shared by drivers used through `database/sql`, including pgx's
stdlib adapter; no enum registration or driver changes are needed. pgx's native
API bypasses `database/sql` and still needs its own scanner adapter or codec.
No SQL methods or database dependency are added to the enum by the compiler.

Representation uses a discriminant and separate typed storage for GC safety.
`reflect.IsEnum`, `reflect.EnumVariants`, `reflect.EnumValueVariant` and
`reflect.EnumValuePayload` expose checked active metadata/payload copies without
changing the existing reflect.Type interface. C calls use explicit adapters.
Ordinary enums require explicitly written serialization adapters with an
application-defined format, as do optionals outside `encoding/json` and
`database/sql`. The opt-in `enum string` text protocol above also enables
standard JSON automatically; native optionals map to JSON `null`/payload and SQL
NULL/payload natively. Neither exposes a stable storage ABI. No zero overhead is
promised.
The source inliner and extraction decline unsupported lazy/contextual changes,
including native optional conversions whose target
would change after extraction. These forms are not automatically rewritten by
`gon fix`.

## Error propagation in tests

```go
func TestLoad(t *testing.T) {
    config := loadConfig("testdata/app.json")!
    if config.Name != "app" { t.Fatalf("name = %q", config.Name) }
}

func load(path string) (Config, error) {
    data := os.ReadFile(path) or err => fmt.Errorf("read %q: %w", path, err)
    return decode(data)!, nil
}
```

In a `_test.go` file, when the nearest function (declaration, literal or lambda)
does not return `error` last, its first parameter can authorize test propagation:
it must be named, not `_`, and have type `*testing.T`, `*testing.B`, `*testing.F`
or `testing.TB` (aliases allowed). A receiver or variadic first parameter does
not qualify. A failing `!` or one-line `or` calls `<param>.Fatal(err)` at the
operator line and returns zero values without `Helper`. This covers tests,
subtests, fuzz callbacks, benchmarks, helpers and lambdas. Block `or` and
optional `?` retain their existing rules. `go/types.TestFatalParam` exposes the
rule to tools.

The gonerrors analyzer suggests `!` for equivalent zero-value error returns
and exact testing Fatal handlers. It suggests `or err => expression` for a
block handler that returns zeros and a contextual error. `testinggoroutine`
models test propagation as an implicit Fatal. `gon explain InvalidErrorHandling`
describes invalid contexts.

## Matching interface and error subjects

```go
type dbError enum {
    default Unknown
    NotFound(string)
    Conflict { Table string; Key int }
}

func (e dbError) Error() string { return "database error" }

func httpStatus(err error) int {
    return switch err {
    case dbError.NotFound(_) => 404
    case dbError.Conflict{Table: table, ...} if table == "users" => 409
    case dbError.Unknown => 500
    default => 500
    }
}
```

A `=>` match on a non-type-parameter interface subject may use qualified enum
variant patterns (unit, positional, record, `...`, nested, with guards;
package-qualified, aliased or generic instances) of an enum whose value type
implements the interface. Otherwise the compiler reports `pattern alternative
can never match interface I: E does not implement I`; an implementation only
through pointer receivers does not count. Patterns also work inside
interface-typed payloads, and value patterns such as `case pkg.Const =>` keep
working.

- For a subject whose type is identical to the predeclared `error`, the subject
  is evaluated once and each arm searches the error tree afresh, in source order,
  with exactly `errors.AsType[E]` semantics: dynamic type, `As(any) bool`,
  `Unwrap() error` and `Unwrap() []error`, depth first. The first `E` found is
  matched against the variant, payload and guard; on failure the next arm is
  tried.
- For any other interface, an arm is a plain `subject.(E)` assertion (exact `E`).
- A nil subject matches no enum arm. Enum arms never cover an interface, so
  `default` or `case _` is required. Statement arms still need braces.

Lowering uses a runtime helper (`matchErrorAs`), so the user's package needs no
`errors` import and the standard library gains no import cycle. One local
measurement found about 12 ns and no allocation per arm on a wrapped error,
against about 82 ns and one allocation for `errors.As`.

## SQL struct scanning

`database/sql` maps a result row to a struct, so the query, the struct and the
`Scan` destination list no longer repeat every column.

```go
type Student struct {
    ID          int
    CreatedAt   time.Time // column created_at
    Email       string?   // NULL is absent
    DisplayName string `sql:"name"`
}

func students(db *sql.DB, course int) ([]Student, error) {
    rows := db.Query("SELECT id, created_at, email, name FROM students WHERE course_id = $1", course)!
    return sql.Collect[Student](rows)
}
```

API: `(*Rows).ScanStruct(dest any) error`, `(*Row).ScanStruct(dest any) error`,
`Collect[T any](*Rows) ([]T, error)`, `CollectOne[T any](*Rows) (T, error)` and
`ErrTooManyRows`.

Composite fields (structs, maps, arrays and non-byte slices) accept JSON string
or byte columns when ordinary SQL conversion is unsupported. This lets queries
return `jsonb_agg`/`to_jsonb` collections straight into `[]T` fields. Native
optional composite destinations also decode JSON through `Scan`: SQL NULL and
JSON null both mean absence. Each JSON conversion starts from a fresh value and
an error leaves that destination unchanged. Explicit Scanners and native scalar
conversions (including time, enum, cursor and decimal conversions) retain
precedence. Plain Go `Scan` destinations keep their existing rules; raw
PostgreSQL array text still needs a codec.

- `Rows.ScanStruct` needs a non-nil pointer to a struct and has the same
  preconditions and errors as `Rows.Scan`. `Row.ScanStruct` is like `Row.Scan`
  (deferred error, `ErrNoRows`, closes the rows) and rejects `RawBytes` fields.
- `Collect` scans the remaining rows of the current result set, always closes
  `rows`, and returns `rows.Err()` (a nil slice on error, a non-nil empty slice
  for no rows). A struct `T` (or `*Struct`, allocated per element) uses the
  mapping; any other `T` receives exactly one column through the normal `Scan`
  conversion. That includes `time.Time`, native string enums, native optionals
  and structs whose `T` or `*T` implements `sql.Scanner`. `RawBytes` as `T` or
  as a field is rejected.
- `CollectOne` returns `ErrNoRows` or `ErrTooManyRows` (`sql: more than one row
  in result`); iteration errors win, the zero `T` is returned on error, and the
  rows are always closed.
- Mapping: an exact `sql:"name"` tag (case-sensitive; a tagged field matches only
  by its tag) wins. An untagged exported field matches a column whose name equals
  the field name after lowercasing and removing `_` (`created_at` to
  `CreatedAt`, `llm_context` to `LLMContext`, `id` to `ID`). `sql:"-"` excludes
  a field; comma options are an error (reserved). An exact tag beats a name
  match at any depth. Embedded structs flatten with `encoding/json` depth
  precedence, and a same-depth tie is an error only if that column is present.
  Embedded tagged structs, `time.Time`, Scanners, enums and optionals are leaves.
  Embedded pointers are allocated when a column maps into them; unexported
  fields are ignored; fields without a column keep their value.
- The mapping is strict: an unmapped, ambiguous or duplicated column, or two
  columns mapping to one field, is an error naming the column and type
  (prefix `sql: <operation>:`), raised before any field is written.
- The per-type field index is cached in a `sync.Map`, and each `Rows` caches its
  plan, invalidated by `NextResultSet` or a different destination type. The
  overhead measured over the fake driver on an Apple M4 is about 60 ns per row
  against a hand-written `Scan`; `Collect` allocates less than the manual loop.

## Executable pairs and validation

Executable legacy/modern pairs live in `test/{enums,stringenums,stringenums_sql,matching,matchinterface,
namedarguments,errorhandling,errortest,optionals,optionsyntax,optionaljson,optionalsql,sqlstruct,
matchalternatives,patterntest,errorcontext,interpolation,seq,nilanalysis}.go` and
their `.dir` folders. The `modern` profile runs the enum, matching (including
`matchinterface`), optional (including `optionaljson` and `optionalsql`),
and named-argument pairs plus the accepted additions; the `errorhandling` profile adds `errorhandling` and
`errortest`; the `tooling` profile also runs `sqlstruct`. Run the deduplicated
focused gate:

```sh
GON_BASELINE_GO=/absolute/path/to/unmodified/go python3 misc/gon/validate.py modern
```

See [the validation record](VALIDATION.md) and [feature status](features.json)
for the executed checks and supported limitations. Snippets above illustrate the
syntax; the executable fixtures supply the regression assertions.

## Build

First build the compiler from the repository's `src` directory with `./make.bash`.
Then, from the repository root:

```sh
python3 misc/gon/build.py
./gon/bin/gon run a.go
./gon/bin/gonpls version
```

The build requires Python 3 and network access for transitive dependencies not
already cached. It builds maintained source directly from `tools/gonpls`,
`tools/x-tools`, `tools/staticcheck` and `tools/nilaway`. Their `UPSTREAM.json` files record source
provenance; `go.mod` and `go.sum` pin transitive dependencies. Builds do not
rewrite maintained modules or apply patches. The old `pkg/gon-tools` trees are
unused disposable output from the previous workflow.

Vet, gonpls and standard-library export readers share the maintained x/tools
source. `src/vendor` and `src/cmd/vendor` are generated
with `python3 misc/gon/vendor.py`; `--check` detects drift. Edit `tools/x-tools`
and regenerate, instead of editing the vendor copies. See
[INTEGRATION.md](INTEGRATION.md) for the integration checklist and per-feature
validation command.

Public executables live in `gon/bin`. The private server lives in
`pkg/tool/<os>_<arch>/gonpls`. Keep that layout intact when moving the toolchain.
The launchers resolve installation symlinks to find their owning toolchain.

`gon` retains upstream command behavior, including `gon version` reporting the
underlying Go version, so tools that query the compiler can still parse it.
`gonpls version` identifies the Gon server and its gopls baseline. Both set
`GOROOT`, prepend the private toolchain directory to their **child process**
`PATH`, and force `GOTOOLCHAIN=local`. A module requiring a newer toolchain must
be handled by upgrading Gon; silently downloading an upstream compiler would
lose Gon syntax support.

## Tooling commands for agents, scripts and CI

`gon query`, `gon refactor`, `gon check`, `gon explain` and `gon capabilities`
are additive commands implemented by the gonpls engine. They need no LSP or MCP
configuration, print compact text or versioned JSON (`--json`), and use stable
exit statuses. Each command analyzes the saved files in its own process and
keeps no state afterwards; editors keep using their own long-lived gonpls.
See [CLI.md](CLI.md) for the contract.

```sh
gon query refs ./store.Sum
gon query type store/store.go:29:16 --json
gon check ./... --json
gon refactor rename store.Sum Add --dry-run
gon explain InvalidErrorHandling
```

Keep the syntax and tooling skill in
[`.agents/skills/gon`](../../.agents/skills/gon/SKILL.md) in every Gon project
so coding agents use the selected language and tools. It is project-scoped;
copy it from the Gon checkout with
`python3 misc/gon/install.py --project-skill /path/to/repo`.

## Install without replacing Go

```sh
python3 misc/gon/install.py
```

The installer creates only `gon` and `gonpls` symlinks in `~/.local/bin`, or in
`--bin-dir /your/chosen/path`. It refuses to overwrite unrelated files. It adds that public directory to PATH persistently when needed, through the
current shell's startup configuration (zsh, bash, fish or sh-compatible shells),
or the user PATH on Windows. Repeated installation preserves existing settings
and does not duplicate the entry. Open a new terminal and restart the editor
after installation; a child process cannot change the current terminal's PATH.
Use `--no-modify-path` to manage PATH yourself. Unrelated commands are never
overwritten; dangling links from a moved Gon checkout can be repaired.
Global Go settings and upstream `go`/`gopls` commands stay unchanged. Never add
this fork's private `bin` directory to PATH. Remove the two installed symlinks
and the Gon PATH block from your shell configuration to uninstall.

In a new terminal, use the public commands:

```sh
gon run a.go
gon fmt ./path/to/package
gonpls check a.go
```

## Adopt Gon in an existing Go project

Start with a built Gon toolchain and keep its directory in place. Expose the
public commands with the installer above, or use absolute paths to
`/path/to/toolchain/gon/bin/gon` and `gonpls` throughout. Adding this fork's
private `bin` directory to PATH would also expose its private `go`; use the
public commands to select Gon for this project.

1. **Run the existing project with Gon.** In your application's module, run:

   ```sh
   cd /path/to/your-project
   gon capabilities --json
   gon test ./...
   gon build ./...
   gon vet ./...
   ```

   Check the reported toolchain root. Keep your `.go` and `_test.go` files,
   `go.mod`, `go.sum`, any `go.work`, dependencies and imports. Selecting Gon
   requires no new language configuration file and does not reinterpret the
   module's `go` directive. If existing code splits prefix negation immediately
   after `!`, put the operand on the same line. See the validation record for
   the supported-platform evidence; your own tests verify your application.

2. **Enable the editor for this workspace.** Install the Gon extension's local
   VSIX, disable the official Go extension for this workspace, reload VS Code,
   and open a `.go` file. The extension selects Gon automatically and discovers
   its installed tools, without `gon.enabled` or a settings file. The
   [VS Code section](#vs-code) describes optional installation overrides.
   Enabling the editor selects Gon for its services;
   terminal and CI commands still need to call `gon` explicitly.

3. **Review syntax modernization separately.** On a branch where you can review
   your changes, preview the supported rewrites:

   ```sh
   gon fix -diff ./...
   ```

   After reviewing the preview, `gon fix ./...` applies those rewrites. To
   restrict the change, pass selected packages or use individual editor quick
   fixes. Format and rerun the affected tests:

   ```sh
   gon fix ./...
   gon fmt ./...
   gon test ./...
   gon vet ./...
   ```

   The supported conversions cover ordinary Go error handling, conditional
   expressions, nil checks and lambdas. Optional values, enums and
   matches are API/design choices you can adopt incrementally. Existing
   `(value, error)` APIs already work with propagation and local handlers;
   retain them when useful partial results accompany an error.

4. **Select Gon in automation.** Provision the same Gon toolchain for CI and
   other developers. Change project build/test/vet/fmt commands to their `gon`
   equivalents and use Gon-aware analysis tools. An unmodified Go parser or
   compiler cannot process packages containing the new syntax. Keep ordinary
   Go dependencies and imports; Gon compiles them as part of the build.

5. **Keep the source transition reviewable.** A project that still contains
   only Go syntax can switch its commands back to Go. After adopting Gon
   syntax, returning to an unmodified Go toolchain also requires reverting or
   rewriting those source changes. `gon fix` modernizes supported forms; it is
   not a reverse migration tool.

For coding agents, include the project-scoped skill from the Gon
checkout with `python3 misc/gon/install.py --project-skill /path/to/your-project`.
This is independent of compiler and editor selection.

## VS Code

The dedicated **Gon** extension (`gon-lang.gon`) is maintained in the separate
`vscode-gon` repository, cloned from `golang/vscode-go`. Install its
local VSIX, disable the official Go extension **for this workspace**, and reload
VS Code. The enabled extension automatically uses `gon` and `gonpls` for `.go`
files. No `gon.enabled` flag, project-enabling command or settings file is required.
For an ordinary Go workspace, disable Gon and enable the official Go extension.
Extension enablement applies to the whole workspace, including all its folders;
use separate windows for projects that need different extensions.

The extension searches PATH and the installer's default `~/.local/bin` public
directory. Its default language server follows the selected compiler's
`gon capabilities` metadata. For a particular installation, optional workspace
settings can override either path:

```json
{
  "gon.compilerPath": "/absolute/path/to/toolchain/gon/bin/gon",
  "gon.languageServerPath": "/absolute/path/to/toolchain/gon/bin/gonpls"
}
```

This toolchain repository overrides the paths to use its development builds.
Other projects normally need no settings. Paths support `~`, `${workspaceFolder}`
and project-relative paths. Tools must be installed in the extension host's
environment (local, SSH, WSL or container).

The extension switches `.go` documents to the **Gon** editor language while it
is enabled, and uses a separate language client per folder. It does not claim
`.go` through a global file association. Gonpls supplies diagnostics, completion,
navigation, formatting, rename and semantic tokens. Optional `gon.serverSettings`
configures the server. Commands use the `gonpls.*` namespace and route to the
correct project when multiple Gon servers are running.

The editor-title **▶ Run Gon File** button saves the active `.go` file and runs
it with Gon. Use `gon.run.mode: "package"` for programs with multiple source files
and `gon.run.args` for program arguments. Output goes to a task terminal. Test
files use `gon test` instead. The extension also provides native Delve DAP
debugging and a test explorer, both using Gon.

The extension neither installs tools automatically nor changes the terminal's
Go environment. Terminal and CI commands must call `gon` explicitly. Run
**Gon: Restart Language Server** after rebuilding gonpls. Remove an old official
Go extension `go.alternateTools` override when adopting this dedicated extension.

## Repository layout

This repository owns the compiler, public launchers and the maintained modules
`tools/gonpls`, `tools/x-tools`, and `tools/staticcheck`, plus the project skill
and build/test scripts. `src/vendor` and `src/cmd/vendor` are generated from selected dependencies
and the maintained x/tools module. The
VS Code extension has its own `vscode-gon` Git repository; its build emits
`gon-0.1.0.vsix`. Neither the local clone nor the VSIX implies publication to
GitHub or the VS Code marketplace.

## Support and current limits

The server uses Gon's parser, type checker and formatter, with adapted AST
traversal, control-flow analysis, interface constraint discovery, and semantic
tokens for `!` and `or`. Diagnostics, hover, definitions, local rename, completion,
formatting, and import organization are covered by the stdio LSP regression.
Normal parse/type errors remain errors; Gon syntax diagnostics are not hidden.

Conditional expressions (`if c { a } else { b }`) also have keyword tokens,
type queries, hover/navigation and completion in both branches and the boolean
condition, including incomplete source. `gon query type` identifies them as
`conditional-expression`. cgo preserves lazy evaluation and contextual argument
types; coverage counts the enclosing statement, with separate counters for
handlers and function bodies. Focused executable pairs check these integrations
against ordinary Go.

Lambdas use `(x) => expression` or `(x) => { statements }` and infer their
signature from the receiving function type. For example:

```go
var twice func(int) int = (x) => x * 2
slices.SortFunc(users, (a, b) => cmp.Compare(a.Name, b.Name))
name := user?.Name ?? "guest"
value := callback?(arg()) ?? 0
config ??= defaults()
```

Safe navigation (`?.`, `?(`) skips the rest of its chain when a Go operand is
nil or a native optional is absent. Coalescing (`??`) evaluates a fallback only
on those absent paths; `??=` stores only on nil or optional absence, according to its
operand type. Zero and empty values remain present, including a present nil payload.
Ordinary interface nil semantics are preserved. Mixed optional/nil chains
require explicit payload extraction, for example `p := optionalPointer ?? nil`
followed by `name := p?.Name ?? "guest"`. Collapsing absence and a present nil payload in the
first step is explicit; parentheses alone do not extract or flatten a payload.
These operators do not add static
non-null types or change ordinary Go nil behavior.

The compiler, public parser/type checker, formatter, cgo, coverage, SSA and
Staticcheck IR understand these constructs. Gonpls supports tokens, inferred
parameter hints, hover, navigation, guarded completion and call signatures.
Lambda/function-literal conversion actions preserve the signature and decline
generic inference or source contexts where that cannot be proved. Converting
a function literal to a lambda currently requires a direct typed declaration
or assignment, without named results.
`gon query type` distinguishes `lambda`, `nil-guard`, `safe-navigation` and
`nil-coalescing`; `gon explain InvalidLambda` and `InvalidNilSafety` describe
their diagnostics.

Source inlining deliberately declines Gon control-flow callee bodies and
affected call sites. Extract-variable actions decline lazy branches and whole
conditional expressions, lambdas and nil-safety expressions, preserving
evaluation order and target conversions.
The existing cgo restriction on propagation/handlers within arguments requiring
pointer-check rewriting also applies inside conditional expressions.

`gon vet`'s `httpresponse` and `sqlrowserr` analyzers look through postfix `!`
and `or` handlers: `resp := client.Do(req)!` before `defer resp.Body.Close()` is
clean, and `rows := db.Query(...)!` is analyzed like the tuple form, while
genuine Go misuse is still reported. The `unreachable` analyzer also inspects
block `or` handlers. Inline variable preserves valid formatting of handler
initializers. `gon query type` identifies ordinary and test error propagation.

The SSA and Staticcheck IR builders lower Gon error expressions to ordinary
branches and returns, including named result resets, `defer`, typed-nil errors,
multiple successful values and returns inside range-over-function loops.
The earlier unsupported-analysis guard has been removed. SSA-based `unusedwrite`
and Staticcheck `SA4006` diagnostics are verified through actual LSP requests.
Enable optional Staticcheck checks with `gon.serverSettings.staticcheck: true`.
The pinned x/tools export reader also supports the compiler's V5 package format.
Upstream crash uploads are not started by Gon.

Other upstream refactorings and optional integrations have not all been validated
with the new syntax. The server is an initial Gon adaptation, not a claim of
complete compatibility with every gopls or third-party analysis feature.

## Regression checks

Use `GON_BASELINE_GO=/absolute/path/to/unmodified/go` with Go 1.27+ for current
validation. Existing records retain the versions actually tested.
The conditional profile includes cgo/bootstrap adapters, coverage, editor query,
extraction and LSP checks; select only checks affected by a change with
`validate.py conditional --only CHECK`. See [VALIDATION.md](VALIDATION.md) for
the exact focused commands used to close integration. The `lambda` and
`nullsafety` profiles add paired execution, feature analyzer diagnostics and
real LSP checks; use `--list` to inspect their focused commands.

Compiler regression inputs under the selected toolchain's `GOROOT/test` are
loaded as standalone files when they begin with a test-harness recipe such as
`// run` or `// errorcheck`. These inputs are independent programs, not one Go
package; upstream gopls otherwise reports spurious duplicate declarations.
Multi-file `*.dir` fixtures and ordinary projects retain normal package loading.
Intentional errors in error-checking fixtures remain visible as diagnostics.
Run `python3 misc/gon/test_workspace.py` to check this behavior over real LSP.

```sh
GON_BASELINE_GO=/path/to/official/go python3 misc/gon/test.py
GON_BASELINE_GO=/path/to/official/go python3 misc/gon/test_cli.py
GON_BASELINE_GO=/path/to/official/go python3 misc/gon/test_analysis.py
GON_BASELINE_GO=/path/to/official/go \
  ./gon/bin/gon test cmd/internal/testdir -run='Test/errorhandling.go$' -count=1
```

The first test executes equivalent legacy and modern programs with Gon and the
legacy program with official Go. It tests inherited-environment isolation and
CLI failure status, then launches `gonpls serve` and makes actual LSP requests.
It checks valid syntax, handler binding types and navigation, rename, completion,
formatting, semantic tokens, import actions, unsaved type errors and their repair.
The analysis test executes equivalent Go/Gon programs with both toolchains,
executes both versions in the SSA interpreter and builds Staticcheck IR with
sanity checks. The final command is the comprehensive language regression.

`test_cli.py` drives the tooling commands through the public launcher: it runs
a legacy/modern pair with Gon and the legacy program with official Go, checks
and renames both, verifies that Go commands keep their behavior and that an
unsaved editor buffer does not reach command-line results. The Go tests of the
commands run from `tools/gonpls` with `gon test ./internal/cmd -run TestGon`;
they cover target resolution, JSON contracts, checks, stale and atomic plans,
formatting and explanations.

## Additions for 2.27

Match arms accept several comma-separated patterns. Every alternative must
bind exactly the same names with identical types; the subject is evaluated once,
then alternatives in written order, and the guard runs once after a match.

`x is P` is a contextual pattern test at comparison precedence. Bindings are
allowed only in an `if` condition or its top-level `&&` operands. They become
available to later operands and the body, and stay out of the else branch.
Irrefutable tests are rejected. Ordinary identifiers named `is` remain valid.

`$"...${expression}..."` and raw interpolated strings format values with `%v`.
A top-level colon introduces one fmt verb, such as `${price:%.2f}`. Plain
braces and dollar signs remain literal; `\$` escapes a dollar in interpreted
text. Expressions may nest literals, composites, named calls and interpolation.
Require an explicit `fmt` import in that file; aliases and shadowing work.
Interpolation produces a nonconstant string and evaluates operands once in
written order. Vet checks format/operand compatibility; the editor offers the
missing import and supports embedded expression services.

`gon/seq` supplies 18 generic functions: Map, Filter, FlatMap, Reduce, GroupBy,
KeyBy, Distinct, ToSet, Find, First, Last, At, Lookup, All, Count, Partition,
MapSeq and FilterSeq. Find/First/Last/At/Lookup return native optionals. Slice
results are never nil. These are functions, with ordinary Go callbacks or Gon
lambdas, and support named slice/map types.

The `gon/` standard-package prefix remains compatible with existing Go
projects. When a selected module, workspace module or vendor tree supplies
`gon/seq`, that package takes precedence. The Gon standard package is used
when no module supplies that import path. Two module providers still produce
the ordinary ambiguous-import error. GOPATH packages also retain precedence.

NilAway is maintained at `tools/nilaway`, with Apache-2.0 license and upstream
provenance. Enable it explicitly with `gon check --nilaway ./...` or editor
`"gon.serverSettings": { "nilaway": true }`; diagnostics are warnings. It uses maintained
Gon flow/type lowering and reports internal analysis errors visibly. Ordinary
nil semantics remain unchanged. See [INTEGRATION.md](INTEGRATION.md) and
[VALIDATION.md](VALIDATION.md) for coverage and execution evidence.
