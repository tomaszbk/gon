# Native optionals

Gon uses `T?` as its native presence/absence type. It has its own identity,
separate from structs, enums and names a program may shadow. `Option`, `Some`
and `None` are ordinary identifiers with no predeclared optional API.

```go
var missing int? = nil
var zero int? = 0
var presentPointer (*User)? = (*User)(nil)

func twice(input int?) int? {
    number := input?
    return number * 2
}

func describe(input int?) string {
    return switch input {
    case nil => "missing"
    case number? => strconv.Itoa(number)
    }
}
```

The zero value is absent. Assignments and explicit conversions preserve an
already assignable optional, turn untyped nil into absence, or lift one
compatible immediate payload. Typed nil, zero, false and empty strings remain
present. There is no recursive lift or flattening. Aliases retain the protocol;
a separately defined type does not. Use `type Maybe[T any] = T?` for an alias.

The suffix binds tightly. `[]int?` is a slice of optional integers; `([]int)?`
is an optional slice. Use `(*User)?` for an optional pointer and `(int?)?` for
nested layers. When inference has no target, `(int?)(7)` constructs a native
optional explicitly. Prefer direct payloads in typed declarations and returns.

## Presence patterns

`P?` requires an optional subject and matches its present payload against `P`.
`case number?` binds a new payload variable. `case nil` covers absence;
`case nil?` matches a present nil payload or a present layer containing an
absent optional. A non-nilable payload rejects `nil?`.

```go
var inner int? = nil
var outer (int?)? = inner
label := switch outer {
case nil => "outer absence"
case nil? => "inner absence"
case (number?)? => strconv.Itoa(number)
}
```

Patterns compose with enum alternatives, literals and record fields. Guards
never establish exhaustive coverage. A presence pattern must cover its payload
domain; `nil` plus an irrefutable `value?` covers the whole optional. Contextual
nil/true/false patterns retain their meanings under ordinary identifier
shadowing. Matching evaluates its input once.

`?` outside a pattern propagates absence to a function returning exactly one
optional. `??` supplies a lazy fallback and `??=` assigns only on absence.
Safe navigation retains its explicit boundary between optional presence and
Go nil checking: a present nil payload is not absence.

## Results and Go boundaries

`Result[T, E]`, `.Ok/.Err`, qualified Result constructors and patterns remain.
`Result[int?, error]` can succeed with absence (`.Ok(nil)`) or a present integer
(`.Ok(7)`). Contextual Result constructors need a fully known expected type.
Result has no implicit bridge to legacy Go tuples, which may carry useful
partial results alongside errors. Existing Go signatures keep their semantics.

## Introspection and representation

`go/types.Optional` exposes `Elem`; `NewOptional`, `OptionalOf` and `IsOptional`
identify native optional types. `EnumOf` returns nil for them. Analysis and
lowering use private storage metadata without public Some/None variants.

Checked reflection uses `reflect.IsOptional`, `OptionalElement`,
`OptionalValuePresent` and `OptionalValuePayload`. Payload extraction returns a
non-addressable copy, preserving original access restrictions and ordinary
reference aliasing. Absent extraction and inappropriate enum/struct-field
operations panic. `reflect.Kind` remains Struct; public field APIs hide storage.
Ordinary formatting prints absence as `nil` and presence as its payload.
Equality, map keys and comparability follow payload types and active presence.

Storage retains a discriminator and typed payload fields for GC and write
barriers. Its size is measured in the current benchmark report. There is no
stable storage ABI or automatic encoding. Serialization and C boundaries need
explicit application-defined adapters.

## Reviewable migration

The current compiler rejects retired optional constructors. The semantic
migration command recognizes them for analysis only, without adding Option to
Universe or changing ordinary compiler/editor checking:

```sh
gon refactor optionals --dry-run ./...
gon refactor optionals --dry-run --json ./... > optional-migration.json
gon refactor apply optional-migration.json
gon test ./...
```

Examples of retired migration input and the current native spelling:

| Retired Gon input | Current Gon |
| --- | --- |
| `var n Option[int]` | `var n int?` |
| `var n Option[int] = .Some(7)` | `var n int? = 7` |
| `var n Option[int] = .None` | `var n int? = nil` |
| `var p Option[*User] = .Some(nil)` | `var p (*User)? = (*User)(nil)` |
| `case Option[int].Some(n)` | `case n?` |
| `case Option[int].None` | `case nil` |
| `case Option[Option[int]].Some(Option[int].Some(n))` | `case (n?)?` |

The plan records each source revision and rejects stale files before writing.
It migrates types, constructors, constructor function values and patterns while
preserving typed nil and nested layers. User-defined homonyms are untouched.
Source must type-check under the retired optional contract. Cases requiring
inaccessible payload type names are declined and need an explicit adapter.
Rewrites that would discard embedded comments are also declined; move the
comment or migrate that declaration manually.
Explicit conversions in the diff preserve expression types and are intentionally
more verbose than manually written native code.

See [STATUS](STATUS.md), [VALIDATION](VALIDATION.md) and the current
[benchmark report](benchmarks/README.md) for executed checks and platform evidence.
