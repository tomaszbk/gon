# Gon commands and language server

`gon` runs this fork's Go-compatible command. `gonpls` runs a Gon-aware build of
gopls. Both pin their subprocesses to this toolchain without replacing the user's
Go installation. Source files still use `.go`.

Gon 2.27 targets backward compatibility with **Go 1.27+**, subject to the
accepted newline-after-prefix-`!` exception. The user selected this support
floor. Use stable Go 1.27.1 as the current
unmodified baseline; the fork's Go 1.28 development version is not a released
Go baseline. Module language directives retain their independent meaning.

`gon fix -diff ./...` previews opportunities to adopt Gon syntax; `gon fix
./...` applies the safe conversions. This includes error propagation/handlers,
conditional expressions, nil-safety operators and contextually typed lambdas.
The same optional hints are available in the editor and with `gon check
--severity=hint ./...`. See [the modernization commands and limits](CLI.md#syntax-modernization).

Current implementation and evidence are consolidated in [STATUS.md](STATUS.md)
and [VALIDATION.md](VALIDATION.md). The [native optional contract](OPTIONALS.md)
describes presence, absence and assisted migration.

## Closed alternatives and named calls

Gon implements contextual `type Name enum` declarations, qualified unit,
positional and record constructors, exhaustive statement/expression matching,
native optional `T?`, shadowable `Result[T, E]`, and named arguments.
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

```go
func parsePort(text string) (port Result[int?, error]) {
    if text == "" { return .Ok(nil) }
    value := strconv.Atoi(text) or problem { return .Err(problem) }
    return .Ok(value)
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

Leading-dot `.Ok(value)` and `.Err(problem)` require a fully known canonical
Result target from a return, declaration, assignment or parameter. Qualified
`Result[T, E].Ok/Err` constructors and patterns remain available. There is no
implicit conversion into Result or between Result and Go error tuples. The
`port` in `(port Result[int?, error])` is a real Go named result visible to
defers, not a descriptive label inside a generic argument.

Enums require qualified variants and an explicit default variant. Record
construction uses field names and Go zeros for omitted fields; partial record
patterns require `...`. New matching is exhaustive. Payload identifiers bind
fresh arm variables; true/false/nil are contextual values even under shadowing.
Use guards to compare outer variables or local constants.

An optional's zero value is absent; Result zero is Ok(zero T). `?` propagates
absence to a function returning one optional. `!` propagates Result failure;
`or problem { ... }` handles it locally. Present nil payloads and Err(nil) retain
their meaning. Optional and legacy nil navigation require an explicit payload
boundary between chains.

For source that used the retired Gon constructors, `gon refactor optionals
./...` previews a semantic migration; `--json` writes a hash-checked plan for
`gon refactor apply`. The analysis recognizes old Gon identities only for this
migration command and leaves user-declared homonyms unchanged. Normal compiler
and editor checking use the current language. See [OPTIONALS.md](OPTIONALS.md).

Named calls use visible signature parameter names. Evaluate callee/receiver
first, then every argument once in written order before passing values in
parameter order. Arguments are associated before generic inference. Positional
prefixes may precede named arguments; an unnamed signature is positional-only.
Named variadics use a final compatible `name: slice...` or omit the parameter.
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
unmarshaling copies the text and replaces the whole value. Standard JSON uses
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
Use standard `sql.Null[Role]` or `*Role` for nullable columns; native `Role?`
does not acquire a SQL mapping. The zero `Unknown("")` is present empty text,
distinct from SQL `NULL`. Explicit `sql.Scanner` and `driver.Valuer` methods
retain precedence. Drivers with direct column scanning receive an internal
Scanner adapter automatically.

This support is shared by drivers used through `database/sql`, including pgx's
stdlib adapter; no enum registration or driver changes are needed. pgx's native
API bypasses `database/sql` and still needs its own scanner adapter or codec.
No SQL methods or database dependency are added to the enum by the compiler.

Representation uses a discriminant and separate typed storage for GC safety.
`reflect.IsEnum`, `reflect.EnumVariants`, `reflect.EnumValueVariant` and
`reflect.EnumValuePayload` expose checked active metadata/payload copies without
changing the existing reflect.Type interface. C calls use explicit adapters.
Ordinary enums and optionals require explicitly written serialization adapters
with an application-defined format. The opt-in `enum string` text protocol above
also enables standard JSON automatically. Neither exposes a stable storage ABI.
No zero overhead is promised.
The source inliner and extraction decline unsupported lazy/contextual changes,
including contextual constructors and native optional conversions whose target
would change after extraction. These forms are not automatically rewritten by
`gon fix`.

Executable legacy/modern pairs live in `test/{enums,stringenums,stringenums_sql,matching,namedarguments,
optionresult,optionsyntax}.go` and their `.dir` folders. Run the deduplicated focused gate:

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
`tools/x-tools` and `tools/staticcheck`. Their `UPSTREAM.json` files record source
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
   expressions, nil checks and lambdas. Optional values, Result, enums and
   matches are API/design choices you can adopt incrementally. Existing
   `(value, error)` APIs already work with propagation and local handlers;
   changing them to Result is optional.

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
Older test-specific environment names remain compatibility aliases. The
conditional profile includes cgo/bootstrap adapters, coverage, editor query,
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
GO_ERROR_HANDLING_BASELINE=/path/to/official/go python3 misc/gon/test.py
GO_ERROR_HANDLING_BASELINE=/path/to/official/go python3 misc/gon/test_cli.py
GO_ERROR_HANDLING_BASELINE=/path/to/official/go python3 misc/gon/test_analysis.py
GO_ERROR_HANDLING_BASELINE=/path/to/official/go \
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
