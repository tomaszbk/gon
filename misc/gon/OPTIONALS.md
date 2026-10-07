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

## Errors and Go boundaries

Use ordinary Go tuples ending in `error` for failure. An optional result can
coexist with an error: `func parse(text string) (int?, error)`. Successful
absence is `nil, nil`; successful presence is a payload and nil; failure has
an error. Existing APIs retain useful partial results for explicit handlers.
Postfix `!` and `or err => expression` propagate errors and reset other result
values before defers. In qualifying `_test.go` functions they call the named
first testing parameter's `Fatal`. See [the error contract](README.md#error-propagation-in-tests).

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
Equality between optional values, map keys and comparability follow payload
types and active presence. Comparing with untyped nil is a presence test:
`x == nil` means absent and `x != nil` means present (either operand order).
It works even for slices, maps and other noncomparable payloads; a present
typed nil is not absence. A shadowed nil identifier keeps its declared type.

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
use the same path. Composite payloads (structs, maps, arrays and slices other
than byte slices) that cannot receive the column through ordinary conversion
also decode JSON string/byte columns into a fresh value. JSON null is absence;
custom JSON methods are honored, and errors preserve the optional. Scanners
retain precedence; strings/bytes keep literal text such as `null`.

`ScanStruct` and struct rows collected by `Collect`/`CollectOne` also decode
JSON columns into ordinary composite fields, so queries can use JSON aggregates
for collections without per-domain Scanners. Ordinary `Rows.Scan` into Go
composites keeps its existing rules. PostgreSQL array text is not a JSON format;
use `jsonb_agg`/`to_jsonb` in the query or a driver-specific adapter.

Not covered: `encoding/xml`, `gob` and other encoders, and pgx's native API
outside `database/sql`, which need their own codecs.

## Syntax and names

Use `T?`, direct payloads, untyped nil and `P?` patterns.
Ordinary semantic rename/apply refactoring remains available. Existing user
declarations named `Option`, `Some`, `None` or `Result` remain ordinary names.

See [STATUS](STATUS.md), [VALIDATION](VALIDATION.md) and the current
[benchmark report](benchmarks/README.md) for executed checks and platform evidence.
