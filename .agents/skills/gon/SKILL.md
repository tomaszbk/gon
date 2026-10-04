---
name: gon
description: Use in repositories that contain this skill, which selected the Gon toolchain. Covers selecting gon and gonpls, implemented error propagation, conditional expressions, lambdas, nil operators, enums, exhaustive matching, native optionals, Result and named arguments, and semantic query, check, refactor and explain commands.
---

# Gon projects

This repository uses **Gon**, a fork of the Go toolchain. Source files are
still `.go`, tests `_test.go`, and modules use `go.mod`/`go.work`. Upstream
`go`, `gofmt` and `gopls` do not understand Gon syntax: in this repository use
`gon` for every Go command and gonpls-based tooling for analysis.

Gon 2.27 preserves compatibility with Go 1.27+ (user decision, 2026-10-02),
subject to the accepted newline-after-prefix-`!` exception. The current
unmodified validation baseline is stable Go 1.27.1. This support floor does
not require rewriting module language directives or historical test records.

## Select the toolchain

- Run `gon` from `PATH`, or the path in `gon.compilerPath` of this project's
  `.vscode/settings.json` if it is set. In the Gon toolchain repository
  itself, use `./gon/bin/gon` after `python3 misc/gon/build.py`.
- Check it with `gon capabilities --json`: it reports the toolchain root,
  the gonpls version and the supported commands. If a command below is missing,
  the toolchain is older; say so instead of falling back to upstream tools.
- `gon build|test|vet|fmt|run|mod|doc|env` behave like the Go commands. Never
  replace them with `go` or install tools globally.

For VS Code, enable the dedicated Gon extension and disable the official Go
extension in this workspace. Gon is selected automatically; no `gon.enabled`
flag or tool-path settings are required. The extension discovers public tools
through PATH or `~/.local/bin`; optional path overrides select development
builds. Terminal/CI commands still use `gon` explicitly. The installer updates
the public PATH persistently when needed; reopen the terminal/editor afterward.

## Implemented language additions

For legacy Go error returns, postfix `!` propagates the error of a call whose
**last result has exactly type `error`**, from a function whose own last result
is `error`:

```go
data := os.ReadFile(path)!          // (data, err) → data, or return on error
name, size := stat()!              // several success results
flush()!                           // error-only call used as a statement
```

On error, the enclosing function returns zero values for its other results
(named results are reset too, before defers run) and the original error. It
never wraps the error and never uses panic/recover.

`call() or err { ... }` handles the error locally. The binding name is
required, scoped to the block and of type `error`:

```go
data := os.ReadFile(path) or err {
    return Config{}, fmt.Errorf("read config %q: %w", path, err)
}
```

Rules for legacy Go error-return calls (code `InvalidErrorHandling`):

- The operand must be a function or method call (not a conversion or value),
  and its last result must be exactly `error` (aliases are fine; named error
  interfaces, concrete error types and type parameters are not).
- `!` needs the nearest enclosing function literal or declaration to return
  `error` last; `or` handlers do not.
- A handler for a call with success results must end in a terminating
  statement (`return`, `panic`, ...). Error-only handlers may fall through.
- No labels, `goto` or labeled `break`/`continue` in handlers; `defer f()!` and
  `go f()!` are invalid (use a function body).
- `!` drops partial results. When a call returns useful values together with
  an error (for example `io.Reader.Read`), keep the explicit
  `n, err := r.Read(p)` form.

Compatibility: a standalone `!` followed by a newline ends the statement.
Never split prefix negation across lines (`valid := !` + newline + `ok` is a
syntax error); write `!ok` or `! ok` on one line. `!=` is unchanged, `or` is
still a valid identifier, and ordinary Go error handling keeps working.

Conditional expressions select one value and evaluate only the chosen branch:

```go
label := if count == 1 { "item" } else { "items" }
data := if cached { readCache()! } else { fetch()! }
```

Both branches are required and must contain one single-valued expression.
No init statements, else-if chains, or direct nested conditional expressions
are allowed. A conditional inside a separate operand (such as a call argument)
is allowed. At statement start, `if` remains an ordinary Go statement.

A contextual target type applies separately to each branch. Without a target,
typed branches must agree; an untyped branch takes the other branch's type.
For interface targets, untyped non-nil branches first acquire their no-target
types; `nil` instead converts directly to the target, preserving nil interfaces.
Explicit conversions distribute over branches. `gon query type` reports the
construct as `conditional-expression`.

Gonpls declines variable extraction from lazy branches and extraction of a
whole conditional, and the source inliner declines Gon control-flow bodies or
affected call sites. These restrictions preserve evaluation and target types.

Lambdas take their parameter and result types from context:

```go
var twice func(int) int = (x) => x * 2
slices.SortFunc(users, (a, b) => cmp.Compare(a.Name, b.Name))
var load func() ([]byte, error) = () => {
    data := os.ReadFile(path)!
    return data, nil
}
```

Parameters must be parenthesized identifiers, without written types. Bodies
may be expressions or ordinary blocks. A lambda has ordinary Go closure and
capture semantics; `return`, `defer`, `!` and `or` inside it belong to that
lambda. A standalone `f := (x) => x` has no target and is invalid. Parentheses
around the whole lambda do not pass a target. Generic calls can infer results
from expression bodies once parameter types are known; block bodies require
known result types. Lambda errors use `InvalidLambda`.
Gonpls offers conversions between lambdas and function literals when signature
identity and source context are safe; it declines generic inference and types
that cannot be written at the current position. Conversion to a lambda requires
a direct typed declaration or assignment without named results.

Null safety operators perform lazy nil checks:

```go
name := user?.Name ?? "guest"
value := callback?(arg()) ?? 0
config ??= defaults()
timeout := *config?.Timeout ?? 30
```

`?.` guards a pointer or interface; `?(` guards a function. A nil guard skips
the remaining primary-expression chain, including arguments and indexes.
Parentheses end that chain. A value-producing chain without `??` must have a
type that can be nil; `??` also accepts guarded non-nilable results and guarded
dereferences. Defaults run only on absence or nil, not on zero, false or empty
values. `??=` evaluates the location once and stores only when its current
value is nil. Ordinary interface nil semantics remain: an interface containing
a typed nil is non-nil. `??` is right associative and cannot mix with other
binary operators without parentheses. Safe chains cannot be assignment targets
or direct `go`/`defer` calls. These are conveniences, not static non-null
guarantees. Diagnostics use `InvalidNilSafety`.

Enums represent closed alternatives, with qualified constructors and one
explicit zero variant:

```go
type Payment enum {
    default Pending
    Rejected(string)
    Paid {
        Receipt string
        Amount  int
    }
}

var waiting Payment // Payment.Pending
failed := Payment.Rejected("expired card")
paid := Payment.Paid{Receipt: "receipt-42", Amount: 5}
```

`enum` is contextual, so existing Go identifiers named enum remain valid.
Positional variants declare types without parameter names; their constructors
can be function values. Record variants require field labels and permit omitted
construction fields with Go zeros. A record constructor head is not a separate
type or a standalone value. Fields and variants follow Go export spelling.
An enum is comparable when all payload types are comparable; equality examines
the active variant and its data. Copies follow Go rules, preserving pointer
aliasing. There is no implicit conversion from legacy tuples or nilable types.

Matching uses `switch` with `=>` arms and requires exhaustive coverage:

```go
label := switch paid {
case Payment.Pending => "pending"
case Payment.Rejected(reason) => reason
case Payment.Paid{Receipt: receipt, ...} => receipt
}

switch failed {
case Payment.Rejected(reason) => {
    fmt.Println(reason)
}
default => {
    fmt.Println("not rejected")
}
}
```

An expression arm yields one value using the conditional's target-type rules;
a statement arm is an ordinary block. Patterns can nest. Payload bindings are
copies local to their arm and its optional `if` guard. The input is evaluated
once; matching selects arms in written order and runs only the selected body.
Guards do not establish exhaustive coverage. `default` or `case _` explicitly
covers remaining alternatives. Partial record patterns require `...`.
Ordinary Go switches with `:` retain their semantics; do not mix `:` and `=>`
arms or use `fallthrough` in a match. Diagnostics use `InvalidMatch`.

Variant patterns require qualification, including nested variants. Bare payload
identifiers declare fresh bindings even when a local constant has the same name.
The three bare names `true`, `false` and `nil` instead denote their contextual
values, including when ordinary Go code shadows those names. Arm bodies, guards
and existing Go syntax retain ordinary identifier lookup. Compare against a local
constant or outer variable with a guard, for example
`case Payment.Rejected(reason) if reason == wanted => reason`. Unqualified
variants, shorthand such as `.Rejected(reason)`, and bare top-level identifiers
are invalid patterns; use a qualified variant, literal, qualified constant or `_`.

Native optionals use `T?`, independently of user identifier names. The language
has no predeclared Option or Some/None constructors. A zero or untyped nil is
absent; a compatible immediate payload is present, including typed nil and zero.
Assignments preserve already assignable optionals and lift at most one layer.
Use aliases (`type Maybe[T any] = T?`) to retain the optional protocol.

```go
func increment(input int?) int? {
    n := input?
    return n + 1
}
func label(input int?) string {
    return switch input {
    case nil => "missing"
    case n? => strconv.Itoa(n)
    }
}
var presentNil (*User)? = (*User)(nil)
var nested (int?)? = (int?)(nil)
```

Presence patterns `P?` examine present payloads. `case nil?` recognizes a present
nil or a present outer layer with an absent inner optional. `(value?)?` extracts
two layers; use parentheses to preserve the `??` coalescing token. Nested
optionals never flatten. `[]int?` is a slice of optional ints; `([]int)?` is an
optional slice. `(int?)(7)` supplies an explicit conversion target.

Outside patterns, postfix `?` propagates absence to a function returning exactly
one optional. `??` defaults lazily on absence and `??=` stores only on absence.
Safe navigation guards presence. It cannot mix with legacy nil guards in one
chain: extract first, such as `p := optionalPointer ?? nil`, then `p?.Name`.
A present nil remains present until that explicit extraction/defaulting choice.

Canonical `Result[T,E]` remains predeclared and shadowable. `.Ok(value)` and
`.Err(problem)` need a fully known expected Result type; qualified constructors
and patterns remain available. Generic calls infer targets from ordinary
arguments or explicit type arguments. They do not invent an error type.

```go
func parsePort(text string) (port Result[int?, error]) {
    if text == "" { return .Ok(nil) }
    number := strconv.Atoi(text) or err { return .Err(err) }
    return .Ok(number)
}
```

Result `!` extracts Ok or returns Err from a function returning exactly one
Result with an assignable error payload. Err(nil) remains failure. `or problem`
binds the payload and requires a terminating handler. Existing Go tuple
propagation remains independent; explicit adapters preserve partial results.
Neither operator uses panic/recover; named results and defers see propagation.

`types.Info.OptionalConversions` records one-layer lifts while `Types` retains
source operand types. Reflection uses checked IsOptional/OptionalElement and
OptionalValuePresent/OptionalValuePayload functions, with detached payload
copies and private storage. Serialization and C need explicit adapters.
See [the current optional contract](../../../misc/gon/OPTIONALS.md).

For retired Gon optional source, use `gon refactor optionals --dry-run ./...`
or its JSON plan followed by `gon refactor apply`. This reuses gonpls and checks
source revisions before writing. Normal compiler/editor checking does not
accept retired constructors. User-defined homonyms remain ordinary Go names.

Named arguments use the declared names of the visible static signature:

```go
func resize(width, height int) {}
resize(height: 480, width: 640)
resize(640, height: 480)

func log(prefix string, values ...int) {}
items := []int{1, 2}
log(prefix: "debug", values: items...)
log(prefix: "debug") // zero variadic elements
```

Evaluate the function or receiver first, then arguments once in written order;
associate them with parameters before generic inference. A positional prefix
may precede named arguments. Duplicate, unknown or missing ordinary arguments,
positionals after a named argument and multi-valued named arguments are invalid.
A named variadic requires a compatible slice expanded as the final argument or
omission for zero elements. Unnamed signatures and parameters named `_` provide
no usable labels. Function-type identity, assignability, positional multi-value
calls, `go` and `defer` retain Go semantics. Parameter rename updates named call
labels; these parameter names are part of the named-call API. No default values,
optional parameters or overloading are added.

The native cores of all nine additions are implemented with maintained tooling.
The user accepted the final 2.27 pattern, mixed-navigation and serialization
policies on 2026-10-03. Use qualified variants, contextual pattern values and
guards for local constants; keep optional/nil boundaries explicit. Enums use a
discriminator and separate typed storage for GC
safety, with measured size overhead for unit-only/zero-sized shapes. Checked
reflection is through `reflect.IsEnum`, `reflect.EnumVariants`,
`reflect.EnumValueVariant` and `reflect.EnumValuePayload`; payloads are detached
copies. External serialization requires explicitly written adapters with an
application-defined format; Gon supplies no automatic enum encoding and no
stable storage ABI. C boundaries also use explicit adapters. First-class
positional constructors currently disable
compiler inlining. Source inlining and extraction conservatively decline
affected named calls, match arms, propagation and lazy/contextual expressions.

In this toolchain repository, read `misc/gon/STATUS.md`, `misc/gon/features.json`
and `misc/gon/VALIDATION.md` for current status and executed checks. Local
specifications live in `$(gon env GOROOT)/design/`; clearly labeled proposals do not change current implementation contracts. Executable pairs live in
`test/{enums,matching,optionresult,namedarguments}.go` and their `.dir` fixtures.
`GON_BASELINE_GO=/absolute/path/to/unmodified/go python3 misc/gon/validate.py modern`
runs the deduplicated focused aggregate. Individual profiles are `enums`,
`matching`, `option`, `result` and `namedarguments`. A partial selection or open
integration contract returns 2; inspect logs instead of claiming a release pass.
Recorded focused runtime validation is darwin/arm64 with Go 1.27.1 legacy
baselines. Portable pairs for all nine features also execute on js/wasm through
Node; linux/amd64 and linux/riscv64 have cross-compilation evidence only.
See the validation record for the scope of each run.

## Work with the tooling

Semantic commands analyze the **saved** files through the gonpls engine, each
in its own process (usually under a second; the first call after rebuilding
the toolchain can take several seconds). Save files before querying them. Add
`--json` for machine-readable output
(schema version 1). Exit status: 0 success, 1 findings, 2 usage, 3 toolchain or
workspace failure.

1. **Understand** before editing:
   - `gon query symbols Name` finds declarations; results are fully qualified
     and usable as targets.
   - `gon query def <target> --doc`, `gon query refs <target>` (always before
     changing a declaration), `gon query impls <target>`,
     `gon query type file.go:line:col` (also shows Gon constructs).
   - Targets: `Name`, `Type.Method`, `pkg.Name`, `./dir.Name`,
     `import/path.Type.Method`, `file.go:line:col` (1-based line, byte column,
     as in compiler errors) or `file.go:#offset`.
   - `gon doc pkg.Symbol` shows documentation for the dependency versions the
     module selects.
2. **Edit**, preferring small consistent changes. For renames use
   `gon refactor rename <target> NewName --dry-run` to preview, then run it
   without `--dry-run`. It refuses to write if files changed since the
   analysis, and it only reformats files that were already gofmt-clean.
3. **Check** each batch of edits: `gon check ./path/... --json`. Fix
   `category: "language"` errors first; `analysis` findings are advisory.
   `gon explain <code>` explains a code such as `InvalidErrorHandling`.
   `gon check` does not build or run tests; its `notVerified` field says what
   remains.
4. **Validate**: format the changed packages and run the relevant focused
   `gon test`/`gon vet` checks. In this toolchain repository follow
   `misc/gon/INTEGRATION.md`, use the maintained module that owns the change,
   and do not run `gon build ./...` from the repository root.

`gon help tooling` and `gon help <command>` list all flags.

## Modernize existing syntax

Use `gon fix -diff ./...` to preview safe Gon syntax conversions and `gon fix
./...` to apply them. `gon check --severity=hint ./...` reports the same optional
suggestions without writes, including edits with `--json`. Gonpls exposes them
as editor hints and quick fixes. The analyzers are `gonerrors` (`!`/`or`),
`gonconditional`, `gonnil` (`??=`, `??`, `?.`, `?(`) and `gonlambda`.
For a focused preview, use `gon fix -gonerrors -diff ./...`; run `gon tool fix
help <analyzer>` for details. `gon fix -diff` exits 1 when it has a diff.

Only recognized, semantics-preserving patterns are converted. Partial-result
handlers, still-used error bindings, unavailable lambda target types and edits
that would discard comments or required imports are conservatively declined.
Ordinary Go constructs remain valid; these hints are not mandatory diagnostics.
