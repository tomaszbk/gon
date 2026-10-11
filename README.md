<p align="center">
  <img src="doc/gon_logo.png" alt="Gon mascot" width="180">
</p>

<h1 align="center">Gon — Go done well.</h1>

<p align="center"><strong>Less boilerplate. Fully retrocompatible.</strong></p>

<p align="center">
  <a href="#see-the-difference">See the difference</a> ·
  <a href="doc/gon-examples.md">Examples</a> ·
  <a href="#documentation">Documentation</a> ·
  <a href="#try-gon">Try Gon</a> ·
  <a href="misc/gon/README.md">Editor setup</a> ·
  <a href="#support-gon">Support Gon</a>
</p>

You know the operation you want to perform. Then come the error checks, nil
checks, temporary variables and callbacks. Gon makes that code shorter while
keeping Go's packages, goroutines and programming model.

**Gon extends the Go compiler directly.** Keep your `.go` files, `go.mod` and
existing Go APIs. Choose Gon per project, alongside your normal Go installation.

## See the difference

Four everyday situations show the core additions. Each Go/Gon pair is compiled
and executed against the same assertions, including failure and absence cases.

### Keep the successful path in view

**Error propagation.** Read a file, add context if it fails, then parse it.

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
context and propagates the resulting error. Both propagation forms require the
nearest function to return `error` last, including in `_test.go` files. Tests
can use a block `or` handler with an explicit `t.Fatal(err)` and `return`.

### Short callbacks, optional access and readable calls

**Lambdas · null-safety operators · conditional expressions · named arguments.**
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

### Closed alternatives, complete handling

**Enums · pattern matching.** A payment has one active state, with data that
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

Gon checks the new match for exhaustive coverage. Add a state, and the compiler
points out matches that need updating. Ordinary Go switches keep their rules.

For textual APIs, `type Role enum string` associates each unit variant with a
constant spelling and keeps unknown text in a default string payload. Gon
supplies `Role.Parse`, `String` and text methods, so JSON needs no per-enum
mapping. Gon's `database/sql` also accepts the enum directly as a query argument
and as a scan destination: `db.Exec(query, role)` and
`db.QueryRow(query).Scan(&role)`. Use `sql.Null[Role]`, `*Role` or a native
`Role?` for SQL `NULL`. This is shared support for drivers using `database/sql`;
pgx's native API is a separate boundary. See the
[string-enum contract](misc/gon/README.md).

`database/sql` also maps rows to structs: `sql.Collect[Student](rows)`,
`sql.CollectOne[Student](rows)` and `ScanStruct` match columns to fields by an
exact `sql:"name"` tag or by field name, strictly, with no per-column `Scan`
argument lists. See [SQL struct scanning](misc/gon/README.md#sql-struct-scanning).

A match on an interface or `error` subject can use enum variants too. On `error`,
each arm searches the wrapped-error tree like `errors.AsType`, so
`case dbError.NotFound(name) =>` finds the variant behind any `%w` wrapping;
`default` is required. See
[matching interface and error subjects](misc/gon/README.md#matching-interface-and-error-subjects).

### A missing value is different from a failed operation

**Optional values (`T?`) · error returns.** Read an optional port number: empty input means no setting;
non-numeric input means parsing failed. A present zero is still a value.

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
individual layers. The language has no predeclared `Option`, `Some` or `None`.
Optionals map natively to JSON `null` and SQL `NULL` in `encoding/json` and
`database/sql`, with no per-field adapters. See [native optionals](misc/gon/OPTIONALS.md#json-and-sql).

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

Download a prebuilt toolchain and the VS Code extension from
[Gon releases](https://github.com/tomaszbk/gon/releases).
[Installation instructions →](doc/install.md)

Use the [development container image](doc/containers.md) to compile and test
Gon projects or build application containers for the cloud:

```sh
docker run --rm ghcr.io/tomaszbk/gon:dev gon version
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

1. **Expose the public commands.** From the Gon checkout root:

   ```sh
   python3 misc/gon/install.py
   ```

   The installer exposes `gon` and `gonpls` and, when necessary, adds their
   public directory to your shell's PATH persistently. Open a new terminal and
   restart VS Code after installation. Your existing `go` stays available.
   Use `--no-modify-path` to manage PATH yourself, or use the absolute public
   tool paths directly. In the new terminal, check the selected installation:

   ```sh
   gon capabilities --json
   ```

2. **Check your existing code with Gon.** Run these in your application module:

   ```sh
   cd /path/to/your-project
   gon test ./...
   gon build ./...
   ```

   Keep `.go` files, `go.mod`, `go.sum`, any `go.work`, imports and dependencies.
   Selecting Gon requires no source rewrite or new configuration file. The
   module's `go` directive keeps its existing meaning. The compatibility
   exception is a prefix `!` split from its operand by a newline; keep it on
   one line.

3. **Select Gon in your editor.** In VS Code, install the Gon extension's
   [downloadable VSIX](https://github.com/tomaszbk/gon/releases), disable the
   official Go extension **for this workspace**, and reload.
   The enabled Gon extension automatically uses Gon for `.go` files and finds
   the installed tools. No `gon.enabled` flag or settings file is required.
   Optional tool-path overrides select a particular installation.
   [Editor installation and workspace settings →](misc/gon/README.md#vs-code)

4. **Adopt the new syntax gradually.** Preview supported conversions, review
   the diff, then apply the ones you want and rerun your tests:

   ```sh
   gon fix -diff ./...
   gon fix ./...
   gon fmt ./...
   gon test ./...
   ```

   `fix` covers supported error handling, conditional expressions, nil operators
   and lambdas. Introduce optional values and enums where your APIs need
   them; existing Go error-returning APIs already work with `!` and `or`.

5. **Use the same toolchain in CI and on teammates' machines.** Provision Gon
   and use `gon build`, `gon test`, `gon vet` and `gon fmt` for the project.
   Packages that use Gon syntax require Gon to compile, analyze and format.
   Go dependencies continue to work through ordinary imports.

**Your editor comes along.** Gonpls provides diagnostics, completion,
formatting, navigation and rename. `gon query`, `gon check` and `gon refactor`
expose the same semantic engine to scripts and agents.
[Full toolchain and editor setup →](misc/gon/README.md)

## Status and compatibility

[Current implementation status](misc/gon/STATUS.md) and
[validation evidence](misc/gon/VALIDATION.md) are maintained in place.

**Gon 2.27 is the first stable release.** The eight original features and accepted 2.27 additions are implemented in
the native compiler and maintained tools. See the [validation record](misc/gon/VALIDATION.md)
for executed checks and platforms, and the [supported tooling limits](misc/gon/features.json).

Gon 2.27 preserves existing **Go 1.27+** source and behavior, with one accepted
exception: prefix `!` cannot be split from its operand by a newline. Write
`!disabled` on one line. New syntax requires Gon; ordinary error returns, nil,
interfaces and Go switches retain their semantics. Go and Gon tools coexist
without replacing or globally shadowing one another.

## Performance, measured

The [22-workload benchmark suite](misc/gon/benchmarks/README.md) compares
unmodified Go 1.27.1, Gon running legacy Go, and Gon running equivalent modern
code with native optionals on Apple M4 / darwin/arm64. Compiler optimizations
remove redundant presence loads, intermediate tags and unused register values.
Runtime costs depend on the workload; no blanket zero-overhead claim is made.

All three variants pass the same 16 correctness tests with identical observable
output. The current report includes every workload, heap allocations, storage
sizes and build costs, including increases. Go/Gon base revisions differ;
modern versus legacy Gon isolates syntax on the same toolchain.
[Measurements, raw samples and limits →](misc/gon/benchmarks/results/20261007T033706Z/report.md)

The following build medians are from that **2026-10-07 source snapshot**, before
the compact unit-enum representation and latest upstream integration. They are
historical measurements, not a fresh benchmark of the release binaries.

| Measurement | Go 1.27.1 | Gon, legacy Go | Gon, modern code | Conclusion |
| --- | ---: | ---: | ---: | --- |
| Recompile and link, dependencies cached | 249 ms | 249 ms | 251 ms | Same |
| Build without changes | 105 ms | 109 ms | 115 ms | Same |
| Executable size, bytes | 3,819,122 | 3,752,418 | 3,752,466 | Smaller |

Here **Same** means similar in scale, not identical: the modern no-change build
was about 9.5% slower than Go in this session. **Smaller** compares the Gon
executables with Go; modern Gon adds 48 bytes over legacy Gon. Builds use warm
dependency caches, and executables include runtime and debug metadata. Go and
Gon have different upstream bases, so these differences cannot all be attributed
to Gon syntax. The full report includes runtime results, allocations,
representation sizes, regressions and measurement limits.

## Support Gon

If Gon helps you, consider supporting its development through
[GitHub Sponsors](https://github.com/sponsors/tomaszbk). One-time and monthly
contributions help fund compiler work, editor tooling, documentation and
compatibility testing.

## Upstream and license

Gon is a native fork of [Go](https://go.dev/), with upstream copyright notices
and [license](LICENSE) preserved.
[Source and tooling provenance](misc/gon/UPSTREAM.json).
