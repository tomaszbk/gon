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
nearest function to return `error` last or qualify as a test below; failure returns
the original error and zeros for other results, including named results, before
defers. Keep explicit handling for useful partial results. Propagation never
uses panic/recover. `or` binds its error locally and must terminate when the
call has success results; error-only handlers may fall through. No
labels/goto/labeled jumps in handlers; no direct `go f()!` or `defer f()!`.

In `_test.go`, when the nearest function does not return `error` last and its
first parameter is a named `*testing.T/B/F` or `testing.TB`, a failing `!` calls
`<param>.Fatal(err)` at that line and returns zeros. This includes tests,
subtests, fuzz callbacks, benchmarks, helpers and lambdas. Receivers, `_` and
variadic first parameters do not qualify. Block `or` and optional `?` keep
their existing contracts.

```go
data := os.ReadFile(path) or err => fmt.Errorf("read %q: %w", path, err)
```

The one-line form evaluates its context only on failure and propagates that
error, with zeros for other results before defers. It is valid wherever `!` is
valid, including qualifying tests. Its binding is local to the context. Use a
block handler to recover, choose partial results or perform several statements.

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
`x == nil`/`x != nil` test absence/presence in either operand order, even for
noncomparable payloads; present typed nil remains present. Shadowed nil keeps
its ordinary type.

`??` defaults lazily on absence/Go nil, never zero/false/empty. `??=` evaluates
the location once and assigns only on absence/nil. `?.` guards presence or
pointer/interface nil; `?(` guards a function. Failure skips the remaining
chain, including arguments/indexes; parentheses end the chain. Without `??`,
a value-producing chain needs a nilable result. Present nil is present; an
interface containing typed nil is non-nil. Mixed optional/nil guards require
extraction first: `p := optionalPointer ?? nil`, then `p?.Name`. Parenthesize
`??` with other binary operators. No safe assignment targets or direct go/defer.

`encoding/json` and `database/sql` need no adapters: absence is `null`/NULL,
presence is the payload; `null`/NULL decodes/scans to absence and a failed
decode/scan leaves the optional unchanged. SQL composite optional payloads
accept JSON string/byte columns, with JSON null as absence; explicit Scanners
keep precedence and string/byte payloads keep their raw SQL representation.
Nested optionals, optional map keys
and RawBytes payloads are rejected. xml/gob and native pgx need codecs.

## Enums and matching

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

func parsePort(text string) (int, error) {
    number := strconv.Atoi(text)!
    return number, nil
}
```

Enums need one `default` zero variant and qualified constructors. Positional
payloads use `Payment.Rejected("reason")`; records require labels, with Go zeros
for omissions. Export spelling and payload comparability follow Go.

Matches are exhaustive: `=> expression` yields a value; `=> { ... }` is a
statement arm. Evaluate the subject once, try arms in order. Qualify variants;
payload identifiers bind new arm-local variables. Record omissions need `...`.
`case n?` matches presence, `nil` absence, `nil?` present nil/absent inner layer,
`(n?)?` two present layers. true/false/nil keep pattern meanings under shadowing.
Compare outer values with guards (`case Payment.Rejected(r) if r == wanted`);
guards do not prove coverage. `default`/`case _` covers the rest. No unqualified
variants, mixed `:`/`=>` or fallthrough; ordinary Go switches keep `:`.
An interface subject (not a type parameter) accepts qualified variants of an enum
implementing it: `case dbError.NotFound(name) =>`. On `error` each arm re-searches
the tree with `errors.AsType[E]` semantics and tries the next arm if the first E
found fails its payload/guard; other interfaces use exact `.(E)`. nil matches no
enum arm; enum arms never cover an interface, so `default`/`case _` is required.
For textual APIs, opt in with `type Role enum string { default Unknown(string);
Teacher = "teacher"; Student = "student" }`. Unit spellings are unique constant
strings. `Role.Parse(text)` preserves unknown text in the default payload; zero
is Unknown(""). `String`, `MarshalText` and pointer `UnmarshalText` are automatic,
so standard JSON needs no methods per enum. Known `UnmarshalText` variants
need no allocation; unknown text is copied so it outlives the input bytes. JSON null preserves an existing
value. Alias/generic/imported enums retain the protocol; `Parse` and the generated
method names cannot be redeclared. String enums remain enum values, so Go string
conversion/JSON tag rules do not implicitly apply. In Gon's `database/sql`, pass
the enum directly to Exec/Query and scan into its pointer: arguments become
strings and string/[]byte results are parsed automatically. SQL NULL requires
`sql.Null[Role]`, `*Role` or `Role?`; a plain Role rejects it without mutation, and
Unknown("") is present empty text. Explicit Scanner/Valuer methods retain
precedence. No enum registration or driver changes are needed through database/sql,
including pgx/stdlib. Native pgx uses a separate API and still needs a codec.
No SQL methods are generated on the enum. Other enum serialization and C
boundaries need explicit adapters.

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
Untyped comparisons, `!`, `&&` and `||` convert to the parameter type: no
`bool(...)`. Parameter rename updates call labels.

## database/sql structs

```go
rows := db.Query("SELECT id, created_at, name FROM students")!
students := sql.Collect[Student](rows)! // []Student; always closes rows
```

Also `sql.CollectOne[T]` (`sql.ErrNoRows`, `sql.ErrTooManyRows`) and
`Rows.ScanStruct`/`Row.ScanStruct`. A column maps to an exported field by exact
`sql:"name"` tag (case-sensitive; tagged fields match only their tag), else by
field name lowercased without `_` (`created_at` to `CreatedAt`, `id` to `ID`);
`sql:"-"` skips; tag options after a comma are errors. Embedded structs flatten
(encoding/json depth rules); `time.Time`, Scanners, enums and optionals are
leaves. Strict: unmapped, ambiguous or duplicate columns error before any write;
unmatched fields keep their value. A non-struct `T` takes one column through
normal Scan conversion. `RawBytes` is rejected. `Collect` returns a nil slice on
error, a non-nil empty one for no rows.

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

## Pattern tests and alternative patterns

```go
if seq.Lookup(users, id) is user? && user.Active { use(user) }
kind := switch payment {
case Payment.Pending, Payment.Rejected(_) => "unpaid"
case Payment.Paid{...} => "paid"
}
```

Several patterns share an arm only when they bind the same names with identical
types. Evaluate the subject once, try alternatives in written order and then
the guard once. `x is P` has comparison precedence and rejects an irrefutable
pattern. Bindings are valid only in an `if` condition or top-level `&&`
operands; later operands and the body can use them, the else branch cannot.
An ordinary identifier named `is` retains Go meaning.

## Interpolation and collections

```go
label := $"Hello ${user.Name}, total ${price:%.2f}"
names := seq.Map(users, (user) => user.Name)
var selected User? = seq.Find(users, (user) => user.Active)
```

Import `fmt` explicitly in the interpolation's file, and `gon/seq` for collection
functions. The fmt import counts as used; aliases and shadowing work. `$"..."`
and raw interpolated strings use `${expression}` with optional top-level
`:verb`. The default is `%v`; one explicit fmt verb may consume the operand,
without `*` or `%%`. Plain braces stay literal. `\$` escapes a dollar in
interpreted text; raw text uses `${"${"}` to emit a literal opener. Interpreted
interpolation forbids newlines even inside expressions. Operand expressions
run once from left to right. The result is a typed nonconstant string. Vet
checks formatting types. Struct tags, import paths and constant contexts reject
interpolation.

`gon/seq` provides Map, Filter, FlatMap, Reduce, GroupBy, KeyBy, Distinct, ToSet,
Find, First, Last, At, Lookup, All, Count, Partition, MapSeq and FilterSeq.
Slice results are never nil. Find/First/Last/At/Lookup return native `T?`;
Lookup distinguishes a stored zero or typed nil from a missing key. Ordinary
Go callbacks and Gon lambdas both work; named slice/map types are supported.

## Nil analysis

NilAway is opt-in: `gon check --nilaway ./...` or editor
`"gon.serverSettings": { "nilaway": true }`. It reports warnings, keeps ordinary Go nil
semantics and uses maintained Gon lowering for new constructs. An internal
analysis error is visible; a no-panic run alone is not evidence that all
nil-flow diagnostics are correct. Review warnings and verify behavior with
project tests. NilAway is maintained in `tools/nilaway` with its Apache-2.0
license and upstream provenance; it is not part of `gon vet`.
