<p align="center">
  <img src="doc/gon_logo.png" alt="Gon mascot" width="180">
</p>

<h1 align="center">Gon — Another way to Go!</h1>

<p align="center"><strong>Less boilerplate. Go-compatible.</strong></p>

<p align="center">
  <a href="#see-the-difference">See the difference</a> ·
  <a href="doc/gon-examples.md">Examples</a> ·
  <a href="#documentation">Documentation</a> ·
  <a href="#try-gon">Try Gon</a> ·
  <a href="misc/gon/README.md">Editor setup</a> ·
  <a href="#support-gon">Support Gon</a>
</p>

The Go features we always wanted, minus the boilerplate we always hated.

**Gon extends the Go compiler directly.** Use Gon to build, test and run your
project. This project is NOT a transpiler.

## See the difference

### **Error propagation.** 
Read a file, add context if it fails, then parse it.

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
	data := os.ReadFile(path) or err => fmt.Errorf("read config %q: %w", path, err)
	config := parseConfig(data)!
	return config, nil
}

```

`!` returns early on failure; `or err { ... }` lets you handle the error yourself.
Both work with existing Go error-returning APIs. `or err => expression` adds
context and propagates the resulting error.

### **Lambdas · null-safety operators · conditional expressions · named arguments.**

Sort users, choose a title and label, then format the list. The helper has the
ordinary signature `formatList(title string, count int, unit string) string`.

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

The callback gets its types from context. `?.` guards a nil owner, `??` supplies
a fallback, and the expression `if` selects a value. Labels use the function's
existing parameter names.

### **Enums · pattern matching.** 
A payment has one active state, with data that
belongs to that state. Handle each alternative without manually coordinating
an integer tag and unrelated fields.

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

Gon checks matches for exhaustiveness: adding a state flags missing cases.
Ordinary Go switches keep their rules.

String enums preserve unknown text and support JSON and `database/sql` directly.
See the [string-enum contract](misc/gon/README.md).

Enum patterns also match interfaces and wrapped errors; a fallback is required.
See
[matching interface and error subjects](misc/gon/README.md#matching-interface-and-error-subjects).

### **Optional values (`T?`) · error returns.**
Empty input means no port; invalid
input means an error. Zero is still a value.

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
func parsePort(text string) (port int?, err error) {
	if text == "" {
		return nil, nil
	}
	number := strconv.Atoi(text)!
	return number, nil
}

func portLabel(text string) string {
	port := parsePort(text) or err {
		return "invalid port"
	}
	return strconv.Itoa(port ?? 8080)
}

```

`int?` means an optional integer: `nil` is absent, while an integer, including
zero, is present. The ordinary `(int?, error)` return keeps absence and failure
separate: `(nil, nil)` has no setting, `(number, nil)` has one, and an error
reports a failed parse.

| Input | Gon return | Display |
| --- | --- | --- |
| `"443"` | `443, nil` | `"443"` |
| `""` | `nil, nil` | `"8080"` |
| `"0"` | `0, nil` | `"0"` |
| `"abc"` | `nil, err` | `"invalid port"` |

`or` handles failure; `??` defaults only on absence. Existing Go error returns
also retain useful partial results when callers handle them explicitly.

`T?` is the native optional type. Match presence with `case value?` and absence
with `case nil`. A typed nil can be present; nested optionals retain their
individual layers.
Optionals map natively to JSON `null` and SQL `NULL` in `encoding/json` and
`database/sql`. See [native optionals](misc/gon/OPTIONALS.md#json-and-sql).

[More examples and boundary cases →](doc/gon-examples.md) ·
[Optional types and ordinary errors →](doc/gon-examples.md#optional-types-and-ordinary-errors)

## More in Gon 2.27

The eight core features above also work with these additions:

| Addition | What it provides | Documentation |
| --- | --- | --- |
| Alternative patterns | Several patterns in one arm: `case A, B =>` | [Patterns](misc/gon/README.md#additions-for-227) |
| Pattern tests | `value is pattern`, with bindings in `if` and later `&&` operands | [Examples](doc/gon-examples.md#collections-pattern-tests-and-interpolation) |
| Error context | `call() or err => fmt.Errorf(...)` | [Error handling](doc/gon-examples.md#errors-retain-the-operation-and-its-context) |
| String interpolation | `$"Hello ${name}"`, including `${price:%.2f}` | [Interpolation](misc/gon/README.md#additions-for-227) |
| Collections | Generic `gon/seq` functions, including optional lookups | [Collections](misc/gon/README.md#additions-for-227) |
| Nil analysis | Opt-in NilAway warnings in `gon check` and gonpls | [NilAway](misc/gon/README.md#additions-for-227) |

String enums supply text, JSON and SQL conversions. Native optionals support
JSON and SQL NULL, and `database/sql` provides strict struct scanning.

## Documentation

Start with [Gon by example](doc/gon-examples.md): executable Go/Gon comparisons
cover the language features, success, errors, absence and evaluation order.

- [Language, commands and editor setup](misc/gon/README.md)
- [Native optional values, JSON and SQL](misc/gon/OPTIONALS.md)
- [Semantic CLI reference](misc/gon/CLI.md)
- [Download and install Gon and the VS Code extension](doc/install.md)
- [Containers and application builds](doc/containers.md)
- [Implementation status](misc/gon/STATUS.md) and [validation evidence](misc/gon/VALIDATION.md)
- [Benchmark methodology](misc/gon/benchmarks/README.md) and [measured results](misc/gon/benchmarks/results/20261007T033706Z/report.md)

## Try Gon

Download a prebuilt toolchain from [Gon releases](https://github.com/tomaszbk/gon/releases)
and install the [Gon extension from the VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=gon-lang.gon).
The release also includes a VSIX for manual installation.
[Installation instructions →](doc/install.md)

Use the [Gon 2.27.0 container image](doc/containers.md) to compile and test
Gon projects or build application containers for the cloud:

```sh
docker run --rm ghcr.io/tomaszbk/gon:v2.27.0 gon version
```

Build from this checkout with an unmodified bootstrap Go toolchain:

```sh
cd src
GOROOT_BOOTSTRAP=/absolute/path/to/bootstrap/go-root ./make.bash
cd ..
python3 misc/gon/build.py
./gon/bin/gon version
```

`GOROOT_BOOTSTRAP` is the directory of an unmodified Go 1.27+ toolchain.
Build Gon once and keep its toolchain directory in place.

### Use Gon in an existing Go project

[Download and install Gon](doc/install.md), set `go 2.27` in your project's
`go.mod`, then build and test:

```sh
cd /path/to/your-project
gon test ./...
gon build ./...
```

`go 2.27` requires Gon 2.27 or newer. Existing `go 1.x` modules keep working.
Use `gon` for your editor and CI too.

For VS Code, install the [Gon extension](https://marketplace.visualstudio.com/items?itemName=gon-lang.gon),
disable the Go extension for this workspace, and reload.

## Status and compatibility

Gon 2.27 supports Go 1.27+ source.
Gon 2.28 will follow the release of Go 1.28.

## Performance, measured

In our [measurements](misc/gon/benchmarks/results/20261007T033706Z/report.md),
rebuild times were comparable and Gon binaries were **1.7% smaller**.

| Measurement | Go 1.27.1 | Gon, legacy Go | Gon, modern code |
| --- | ---: | ---: | ---: |
| Recompile and link, dependencies cached | 249 ms | 249 ms | 251 ms |
| Build without changes | 105 ms | 109 ms | 115 ms |
| Executable size, bytes | 3,819,122 | 3,752,418 | 3,752,466 |

## Support Gon

If Gon helps you, consider supporting its development through
[GitHub Sponsors](https://github.com/sponsors/tomaszbk).

## Upstream and license

Gon is a native fork of [Go](https://go.dev/), with upstream copyright notices
and [license](LICENSE) preserved.
[Source and tooling provenance](misc/gon/UPSTREAM.json).
