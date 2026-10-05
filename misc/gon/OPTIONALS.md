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
Result has no implicit conversion to or from legacy Go tuples, which may carry
useful partial results alongside errors. Existing Go signatures keep their
semantics. Postfix `!` is the one bridge: it fails a Go call ending in exactly
`error` as `Result[T, E].Err(err)` in a function whose only result is a Result
accepting `error`, and fails a `Result[V, E]` with `E` assignable to `error` as
its payload in a function whose last result is `error`. A nil `Err` payload becomes `errors.ErrNilResult`,
because a failed Result never turns into a nil error. In `_test.go` files with a
named first `*testing.T`, `*testing.B`, `*testing.F` or `testing.TB` parameter,
`!` reports the error or Result payload with `Fatal`. See
[README.md](README.md#error-propagation-across-tests-tuples-and-result).

## Introspection and representation

`go/types.Optional` exposes `Elem`; `NewOptional`, `OptionalOf` and `IsOptional`
identify native optional types. `EnumOf` returns nil for them. Analysis and
lowering use private storage metadata without public Some/None variants.

Checked reflection uses `reflect.IsOptional`, `OptionalElement`,
`OptionalValuePresent`, `OptionalValuePayload` and `OptionalValueSetPayload`.
Payload extraction returns a non-addressable copy, preserving original access
restrictions and ordinary reference aliasing. `OptionalValueSetPayload(v,
payload)` makes a settable optional present with a copy of an assignable
payload; it panics for a non-optional, unaddressable, read-only or
non-assignable operand. Absent extraction and inappropriate enum/struct-field
operations panic. `reflect.Kind` remains Struct; public field APIs hide storage.
Ordinary formatting prints absence as `nil` and presence as its payload.
Equality, map keys and comparability follow payload types and active presence.

Storage retains a discriminator and typed payload fields for GC and write
barriers. Its size is measured in the current benchmark report. There is no
stable storage ABI. Only `encoding/json` and `database/sql` encode optionals
automatically (next section); other serializers, native pgx and C boundaries
need explicit application-defined adapters.

## JSON and SQL

Absence is JSON `null` or SQL NULL; presence is the payload's own encoding. The
optional storage is never exposed: both packages use the checked reflect API.

`encoding/json` (the default v2-backed implementation and the `nojsonv2` v1
implementation) marshals absence as `null` at the top level, in fields, elements
and map values. A present value encodes as its payload, keeping a present zero,
and honors the payload's `Marshaler`/`TextMarshaler`, including pointer
receivers. `omitempty` and `omitzero` omit only absence; the native
`encoding/json/v2` API instead applies v2's definition of an empty JSON value to
`omitempty`, which also omits a present empty string. `,string` applies to the
payload. Unmarshaling `null` makes the optional absent and never calls a payload
`Unmarshaler`. Any other input decodes into a temporary, starting from the
existing payload when present, and stores presence only on success: an error
leaves the optional unchanged, and a missing field is untouched. Rejected:
nested optionals, optional map keys, and a present payload whose encoding is
`null` (`UnsupportedValueError`; the native v2 API encodes a nil slice or map as
`[]` or `{}` and does not hit this case).

`database/sql` binds an optional argument (or a pointer to one, including
`sql.Named`) before any `NamedValueChecker` or `ColumnConverter` runs: absent or
a nil pointer is NULL; present is converted as the payload would be (string
enums, `driver.Valuer`, default conversion). `driver.DefaultParameterConverter`
does the same; a nested optional is an error. Scanning into a `*T?` stores
absence for NULL; otherwise it converts into a fresh `T` with the ordinary rules
(string enums, uuid, a Scanner payload) and stores presence only on success, so
an error leaves the destination unchanged. A Scanner payload is never called for
NULL. Nested optionals and `RawBytes` payloads are rejected; `sql.Null[T]` and
pointers are unchanged. Unlike a legacy `*T`, which allocates before a failed
scan, a `T?` stays unchanged. `ScanStruct` fields and a scalar `T` of `Collect`
use the same path.

Not covered: `encoding/xml`, `gob` and other encoders, and pgx's native API
outside `database/sql`, which need their own codecs.

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
