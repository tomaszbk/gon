# Gon by example

[Back to Gon](../README.md)

These are equivalent executable scenarios. The [example fixtures](../misc/gon/readme-examples/)
include the supporting types/functions and assertions; every displayed Go block
is checked against its compiled source. They cover success, failure, absence,
useful partial results, capture, lazy branches and written argument order.

## Errors: retain the operation and its context

Reading and parsing a configuration usually repeats the same return protocol.

**Go**

<!-- readme-example: error-context legacy -->
```go
func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	config, err := parseConfig(data)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}
```

**Gon**

<!-- readme-example: error-context modern -->
```go
func loadConfig(path string) (Config, error) {
	data := os.ReadFile(path) or err {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	config := parseConfig(data)!
	return config, nil
}
```

`!` returns the original error and zero values for the other enclosing results.
`or err { ... }` provides a local handler, such as wrapping that error with
`%w`. Both lower to ordinary control flow and evaluate the call once. This
example deliberately discards the parser's partial result on failure in both
versions. When values remain useful alongside an error, as with
`io.Reader.Read`, keep the ordinary `n, err := reader.Read(buffer)` contract.

## Nil: default only when the receiver is absent

**Go**

<!-- readme-example: nil-default legacy -->
```go
func displayName(user *User) string {
	name := "guest"
	if user != nil {
		name = user.Name
	}
	return name
}
```

**Gon**

<!-- readme-example: nil-default modern -->
```go
func displayName(user *User) string {
	return user?.Name ?? "guest"
}
```

Safe navigation skips the guarded tail on nil; the default runs only when
needed. An existing user whose name is empty still produces an empty name.
`?(` guards nullable functions and `??=` initializes only an absent location.
These conveniences preserve Go's nil/interface behavior; they do not introduce
static non-null types.

## Optional values (`T?`): a value, or nothing

`string?` means “a string may be available.” Think of Python's
`str | None`: a lookup can find a nickname, or find nothing. Go often represents
this with two returns, `(string, bool)`; `string?` puts both possibilities in one
value.

- `"Ada"`: a nickname was found.
- `""`: a nickname was found, and it is empty.
- `nil`: no nickname was found.

`?? "guest"` supplies a fallback only when nothing was found. It does not
replace an empty string.

**Go**

<!-- readme-example: option legacy -->
```go
func nickname(user User) (string, bool) {
	return user.Nickname, user.HasNickname
}

func greeting(user User) string {
	name, ok := nickname(user)
	if !ok {
		return "guest"
	}
	return name
}
```

**Gon**

<!-- readme-example: option modern -->
```go
func nickname(user User) (name string?) {
	if !user.HasNickname {
		return nil
	}
	return user.Nickname
}

func greeting(user User) string {
	return nickname(user) ?? "guest"
}
```

An optional value's zero is absence. Return a value directly when it is present,
or untyped `nil` when absent. `?` propagates absence from a function returning
one optional value; `??` supplies a fallback only on absence. Zero and empty
payloads stay present. Nested optional values remain distinct; switching from
optional presence to a payload's ordinary nil handling requires explicit
extraction. The [type model and explicit variants](#contextual-construction-and-optional-types)
explain the boundary cases and matching syntax.

## Enums and matching: one active alternative, complete handling

**Go**

<!-- readme-example: alternatives legacy -->
```go
type paymentState int

const (
	pending paymentState = iota
	paid
	rejected
)

type Payment struct {
	state   paymentState
	Receipt string
	Reason  string
}

func describePayment(payment Payment) string {
	switch payment.state {
	case pending:
		return "pending"
	case paid:
		return "paid: " + payment.Receipt
	case rejected:
		return "rejected: " + payment.Reason
	default:
		panic("invalid payment state")
	}
}
```

**Gon**

<!-- readme-example: alternatives modern -->
```go
type Payment enum {
	default Pending
	Paid { Receipt string }
	Rejected(string)
}

func describePayment(payment Payment) string {
	return switch payment {
	case Payment.Pending => "pending"
	case Payment.Paid{Receipt: receipt} => "paid: " + receipt
	case Payment.Rejected(reason) => "rejected: " + reason
	}
}
```

Unit, positional and record variants share one closed enum model. Every enum
names an explicit `default` variant for its Go-compatible zero. Record
construction uses field labels; record patterns must name their fields or
explicitly ignore the remainder with `...`.

The new `switch`/`=>` match is exhaustive: adding a variant requires handling
it or deliberately using a catch-all. Payload bindings are local copies in
the selected arm, guards run in source order, and unselected bodies stay lazy.
Ordinary Go switches retain their existing rules.

## Result: a value, or a reason it failed

A payment operation either succeeds with a `Payment` or fails with a reason.
`Result[Payment, string]` represents those two possibilities in one value:

- `Ok(payment)` is the success alternative, carrying a `Payment`.
- `Err("negative amount")` is the failure alternative, carrying a `string`.

These are alternative constructors, not exceptions. In code they need their
type name in front. A Go type alias gives the operation a short, readable name:
`type ChargeResult = Result[Payment, string]`. Then
`ChargeResult.Err("negative amount")` means “create a failed charge with this
reason.” The alias names the same type; it adds no wrapper or conversion.

Unlike Go's `(Payment, error)`, a Result cannot carry a success payload and a
failure payload at the same time. Use conventional Go returns when useful
partial results can accompany an error. You can use Gon's error propagation
with existing Go APIs without adopting Result.

**Go**

<!-- readme-example: result legacy -->
```go
func charge(amount int) (Payment, error) {
	if amount < 0 {
		return Payment{}, errors.New("negative amount")
	}
	return newPaid(fmt.Sprintf("receipt-%d", amount)), nil
}

func paymentStatus(amount int) string {
	payment, err := charge(amount)
	if err != nil {
		return "failed: " + err.Error()
	}
	return describePayment(payment)
}
```

**Gon**

<!-- readme-example: result modern -->
```go
type ChargeResult = Result[Payment, string]

func charge(amount int) (payment ChargeResult) {
	if amount < 0 {
		return .Err("negative amount")
	}
	return .Ok(newPaid(fmt.Sprintf("receipt-%d", amount)))
}

func paymentStatus(amount int) string {
	payment := charge(amount) or reason {
		return "failed: " + reason
	}
	return describePayment(payment)
}
```

`Result[T, E]` gives an API an exclusive success/failure contract with a typed
error payload. The example chooses a string domain error; `E` can also be
`error` or another type. `Err(nil)` remains an error variant. Its 2.27 zero is
`Ok(zero T)`. `!` propagates to a compatible Result return, while `or` handles
it locally. Go tuple returns remain separate: only postfix `!` bridges them (a
failing call ending in `error` becomes `.Err(err)` in a Result function, and a
failed Result returns its payload as the error from an `error`-returning
function); Gon does not silently convert them or lose their partial data.

## Lambdas: keep the closure, shorten the callback

**Go**

<!-- readme-example: lambda legacy -->
```go
func sortUsers(users []User) {
	slices.SortFunc(users, func(a, b User) int {
		return cmp.Compare(a.Name, b.Name)
	})
}
```

**Gon**

<!-- readme-example: lambda modern -->
```go
func sortUsers(users []User) {
	slices.SortFunc(users, (a, b) => cmp.Compare(a.Name, b.Name))
}
```

Parameter and result types come from context. Lambdas have ordinary Go closure,
return and defer behavior; they can also have block bodies. Function literals
remain available when explicit types or a longer body are clearer.

## Conditional expressions: select one value lazily

**Go**

<!-- readme-example: conditional legacy -->
```go
func itemLabel(count int) string {
	label := "items"
	if count == 1 {
		label = "item"
	}
	return fmt.Sprintf("%d %s", count, label)
}
```

**Gon**

<!-- readme-example: conditional modern -->
```go
func itemLabel(count int) string {
	label := if count == 1 { "item" } else { "items" }
	return fmt.Sprintf("%d %s", count, label)
}
```

Both branches are required and produce one value. Only the chosen branch runs;
statement `if` remains unchanged.

## Named arguments: show the association without changing the signature

For `func resize(width, height int) string`, a call may name and reorder its
arguments while keeping their evaluation in written order.

**Go**

<!-- readme-example: named-arguments legacy -->
```go
func previewSize() string {
	height := dimension("height", 480)
	width := dimension("width", 640)
	return resize(width, height)
}
```

**Gon**

<!-- readme-example: named-arguments modern -->
```go
func previewSize() string {
	return resize(height: dimension("height", 480), width: dimension("width", 640))
}
```

Both versions evaluate height before width. Names come from the visible static
signature, including function values and interface methods. Positional prefixes
may precede named arguments. There are no defaults, optional parameters or
added overloading. Named variadics accept a final `name: slice...`; function-type
identity and existing positional calls keep Go's rules.

## Several additions in one function

The list example combines lambdas, nil navigation/defaulting, conditional
expressions and named arguments. Supporting types and `formatList` live in the
[shared fixture](../misc/gon/readme-examples/common.go).

**Go**

<!-- readme-example: user-list legacy -->
```go
func userList(users []User, owner *User) string {
	slices.SortFunc(users, func(a, b User) int {
		return cmp.Compare(a.Name, b.Name)
	})
	title := "guest"
	if owner != nil {
		title = owner.Name
	}
	unit := "users"
	if len(users) == 1 {
		unit = "user"
	}
	return formatList(title, len(users), unit)
}
```

**Gon**

<!-- readme-example: user-list modern -->
```go
func userList(users []User, owner *User) string {
	slices.SortFunc(users, (a, b) => cmp.Compare(a.Name, b.Name))
	title := owner?.Name ?? "guest"
	unit := if len(users) == 1 { "user" } else { "users" }
	return formatList(title: title, count: len(users), unit: unit)
}
```

## Optional settings that can fail to parse

An empty port setting is absent; an invalid integer is a failure. Successful
parsing produces a present integer, including zero. This example combines
Optional values and Result without collapsing their distinct alternatives.

**Go**

<!-- readme-example: optional-port legacy -->
```go
func parsePort(text string) (int, bool, error) {
	if text == "" {
		return 0, false, nil
	}
	port, err := strconv.Atoi(text)
	if err != nil {
		return 0, false, err
	}
	return port, true, nil
}

func portLabel(text string) string {
	port, found, err := parsePort(text)
	if err != nil {
		return "invalid port"
	}
	if !found {
		port = 8080
	}
	return strconv.Itoa(port)
}
```

**Gon**

<!-- readme-example: optional-port modern -->
```go
func parsePort(text string) (port Result[int?, error]) {
	if text == "" {
		return .Ok(nil)
	}
	number := strconv.Atoi(text) or err {
		return .Err(err)
	}
	return .Ok(number)
}

func portLabel(text string) string {
	port := parsePort(text) or err {
		return "invalid port"
	}
	return strconv.Itoa(port ?? 8080)
}
```

## Contextual construction and optional types

The executed optional-port example uses `int?` for an optional integer. In its
`Result[int?, error]` return, `.Ok(nil)` succeeds without a setting,
`.Ok(number)` succeeds with a present number, and `.Err(err)` reports failure.
The named result supplies the expected type. A `Result[int, error]` instead
contains an ordinary integer in its success alternative.

`T?` is a native optional type. Its zero value and untyped `nil` represent
absence. An immediate compatible payload becomes present, including zero,
false and an empty string. A typed nil remains present: use `(*User)(nil)`
when assigning a present nil pointer to `(*User)?`. Already compatible
optional values are preserved. There is no recursive lift or flattening.

Match absence with `case nil` and presence with `case value?`. Presence
patterns can nest: `case nil?` recognizes a present outer layer containing
an absent inner layer; `case (value?)?` extracts two present layers.
`Option`, `Some` and `None` have no special language meaning. Existing Go
programs may still define identifiers with those names.

The suffix binds tightly: `[]int?` is a slice of optional ints, while `([]int)?`
is an optional slice. Use `(*User)?` for an optional pointer and `(int?)?` for
nested optionals. The existing `??` token retains its coalescing meaning.
An explicit conversion, such as `(int?)(7)`, supplies a target when inference
has none. Prefer direct values in typed declarations, assignments and returns.

A contextual constructor needs an expected canonical Result type. `value := .Ok(3)`
lacks an error type and is rejected; a typed declaration, assignment, return,
or known call parameter supplies the target. Generic calls use explicit type
arguments and ordinary argument inference before checking contextual
constructors. They never invent an error type from success alone. Implicit
lifting accepts single-valued expressions and never recursively adds layers.

Ordinary named results, such as `(port Result[int?, error])`, communicate the
result's role using existing Go syntax and create a real result variable. Gon
does not add labels inside Result type arguments.

## Run these examples

From the repository root, with a separate unmodified Go 1.27+ toolchain:

```sh
GON_BASELINE_GO=/absolute/path/to/unmodified/go python3 misc/gon/test_readme.py
```

The check compiles and executes legacy source with Go and Gon, and modern
source with Gon. It also checks every displayed Go block in this guide and the
README against those source fixtures. Supporting definitions and assertions
live in the [fixtures](../misc/gon/readme-examples/).
