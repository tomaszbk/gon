---
name: gon
description: Write, read, test and refactor Gon code using its current syntax, gon commands and gonpls. Use for development in projects that select the Gon toolchain.
---

# Gon 2.27

Keep this skill at `.agents/skills/gon/SKILL.md` in every Gon project.
Gon extends Go natively; keep `.go`, `_test.go`, `go.mod` and `go.work`.
Go 1.27+ remains compatible, including ordinary errors and nullable types,
except that a newline after `!` ends the statement. Keep prefix negation on one line.

## Tools and editor

Use the project's `gon` for build, test, run, vet, fmt, mod, doc and env;
use `gonpls` for language services. Select public tools from PATH or project
settings (`gon.compilerPath`, `gon.languageServerPath`). Keep upstream Go
installed separately; do not put Gon's private `bin` directory on PATH.
Check selection/support with `gon capabilities --json`; report missing capabilities.

VS Code: enable Gon (`gon-lang.gon`), disable Go for the workspace. Other LSP
editors launch `gonpls`. It supplies diagnostics, completion, navigation, rename,
formatting and quick fixes. Terminal/CI call `gon` explicitly.

## Error handling

```go
data := os.ReadFile(path)! // extract data, or return the error
flush()!                  // error-only call
name, size := stat()!      // multiple success results

data := os.ReadFile(path) or err {
    return Config{}, fmt.Errorf("read config: %w", err)
}
```

The call must return exactly `error` last (aliases allowed). `!` requires the
nearest function to return `error` last; failure returns the original error and
zeros for other results, including named results, before defers. Keep explicit
handling for useful partial results. Propagation never uses panic/recover.
`or` binds its error locally and must terminate when the call has success
results; error-only handlers may fall through. No labels/goto/labeled jumps
in handlers; no direct `go f()!` or `defer f()!`.

## Conditional expressions and lambdas

```go
label := if count == 1 { "item" } else { "items" }
var twice func(int) int = (x) => x * 2
slices.SortFunc(users, (a, b) => cmp.Compare(a.Name, b.Name))
var load func() ([]byte, error) = () => {
    return os.ReadFile(path)
}
```

Conditionals require two single-expression branches; only the chosen one runs.
The target types both branches; otherwise typed branches must agree. No init,
else-if or direct nesting. At statement start, `if` remains a Go statement.
Lambdas need a function target: `f := (x) => x` is invalid. Parameters have no
written types. Generic expression bodies can infer results once parameters are
known; block results must be known. Captures, return and defer follow Go closures.

## Optionals and nil safety

```go
var missing int? = nil
var present int? = 0
var presentNil (*User)? = (*User)(nil)
var nested (int?)? = (int?)(nil)

func increment(input int?) int? {
    n := input? // return absence if input is absent
    return n + 1
}

name := user?.Name ?? "guest"
value := callback?(arg()) ?? 0
config ??= defaults()
```

`T?` zero/untyped nil means absence; payloads, even typed nil/zero/false, are
present. Assignment/conversion preserves assignable optionals or lifts one
immediate payload; no flattening. `(int?)(7)` is an explicit conversion.
`[]int?` means optional elements; `([]int)?` means optional slice. Use aliases
like `type Maybe[T any] = T?`; no predeclared Option/Some/None API. Postfix `?`
returns absence early from the nearest function, which must return one optional.

`??` defaults lazily on absence/Go nil, never zero/false/empty. `??=` evaluates
the location once and assigns only on absence/nil. `?.` guards presence or
pointer/interface nil; `?(` guards a function. Failure skips the remaining
chain, including arguments/indexes; parentheses end the chain. Without `??`,
a value-producing chain needs a nilable result. Present nil is present; an
interface containing typed nil is non-nil. Mixed optional/nil guards require
extraction first: `p := optionalPointer ?? nil`, then `p?.Name`. Parenthesize
`??` with other binary operators. No safe assignment targets or direct go/defer.

## Enums, Result and matching

```go
type Payment enum {
    default Pending
    Rejected(string)
    Paid { Receipt string; Amount int }
}

paid := Payment.Paid{Receipt: "receipt-42", Amount: 5}
label := switch paid {
case Payment.Pending => "pending"
case Payment.Rejected(reason) => reason
case Payment.Paid{Receipt: receipt, ...} => receipt
}

func describe(input int?) string {
    return switch input {
    case nil => "missing"
    case n? => strconv.Itoa(n)
    }
}

func parsePort(text string) Result[int, error] {
    number := strconv.Atoi(text) or err { return .Err(err) }
    return .Ok(number)
}
```

Enums need one `default` zero variant and qualified constructors. Positional
payloads use `Payment.Rejected("reason")`; records require labels, with Go zeros
for omissions. Export spelling and payload comparability follow Go.

`Result[T,E]` is shadowable; zero is Ok(zero T), Err(nil) is failure.
`.Ok`/`.Err` need a fully known target; otherwise qualify, e.g.
`Result[int,error].Ok(7)`. Result `!` extracts Ok or propagates Err to exactly
one Result return with an assignable error payload. `or problem` binds the
payload and must terminate. Go error tuples need explicit Result adapters.

Matches are exhaustive: `=> expression` yields a value; `=> { ... }` is a
statement arm. Evaluate the subject once, try arms in order. Qualify variants;
payload identifiers bind new arm-local variables. Record omissions need `...`.
`case n?` matches presence, `nil` absence, `nil?` present nil/absent inner layer,
`(n?)?` two present layers. true/false/nil keep pattern meanings under shadowing.
Compare outer values with guards (`case Payment.Rejected(r) if r == wanted`);
guards do not prove coverage. `default`/`case _` covers the rest. No unqualified
variants, mixed `:`/`=>` or fallthrough; ordinary Go switches keep `:`.
Enums/optionals need explicit serialization and C adapters.

## Named arguments

```go
func resize(width, height int) {}
resize(height: 480, width: 640)
resize(640, height: 480)

func log(prefix string, values ...int) {}
log(prefix: "debug", values: items...)
log(prefix: "debug") // zero variadic elements
```

Labels come from the visible static signature, including function values and
interface methods. Evaluate function/receiver first, then arguments once in
written order. Positionals only precede names; named values are single-valued.
Named variadics take a slice expanded last or omission. Unnamed/`_` parameters
have no labels. No defaults or overloading; function-type identity is unchanged.
Parameter rename updates call labels.

## Semantic commands

Run in the relevant module. CLI analysis uses gonpls on saved files, separately
from the editor session. Prefer semantic commands for Gon symbols/types:

| Command | Use |
| --- | --- |
| `gon query symbols Name` | Find fully qualified symbol targets |
| `gon query def <target> --doc` | Declaration and documentation |
| `gon query refs <target>` | References before changing a declaration |
| `gon query impls <target>` | Interface implementations |
| `gon query type file.go:line:col` | Types and Gon constructs |
| `gon refactor rename <target> NewName --dry-run` | Preview rename; omit `--dry-run` to apply |
| `gon refactor apply plan.json` | Apply a JSON plan; stale files are rejected |
| `gon check ./path/... --json` | Syntax, types and analyzer findings |
| `gon explain InvalidErrorHandling` | Explain a diagnostic code |
| `gon fix -diff ./path/...` | Preview safe syntax modernization; omit `-diff` to apply |

Targets accept `Name`, `Type.Method`, `pkg.Name`, `./dir.Name`, imported symbol
paths, `file.go:line:col` (1-based, byte columns) or `file.go:#offset`.
Semantic commands accept `--json`. Exits: 0 success, 1 findings, 2 usage,
3 workspace/toolchain failure. `gon fix -diff` exits 1 for a diff; `gon check --severity=hint` shows
optional modernization hints. Checking does not build/run tests: follow edits
with `gon fmt`, focused `gon test` and relevant `gon vet`. Inlining/extraction
may decline named/lazy/contextual constructs. Flags: `gon help tooling` or
`gon help <command>`.
