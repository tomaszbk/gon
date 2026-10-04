<p align="center">
  <img src="doc/gon_logo.png" alt="Gon mascot" width="180">
</p>

<h1 align="center">Gon — Go done well.</h1>

<p align="center"><strong>Less boilerplate. Fully retrocompatible.</strong></p>

<p align="center">
  <a href="#see-the-difference">See the difference</a> ·
  <a href="doc/gon-examples.md">Examples</a> ·
  <a href="#try-gon">Try Gon</a> ·
  <a href="misc/gon/README.md">Editor setup</a>
</p>

You know the operation you want to perform. Then come the error checks, nil
checks, temporary variables and callbacks. Gon makes that code shorter while
keeping Go's packages, goroutines and programming model.

**Gon extends the Go compiler directly.** Keep your `.go` files, `go.mod` and
existing Go APIs. Choose Gon per project, alongside your normal Go installation.

## See the difference

Four everyday situations show all nine additions. Each Go/Gon pair is compiled
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
	data := os.ReadFile(path) or err {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	config := parseConfig(data)!
	return config, nil
}
```

`!` returns early on failure; `or err { ... }` lets you handle the error yourself.
Both work with existing Go error-returning APIs.

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

### A missing value is different from a failed operation

**Optional values (`T?`) · Result.** Read an optional port number: empty input means no setting;
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

`int?` means an optional integer: `nil` is absent, while an integer, including
zero, is present. `Result[int?, error]` adds success or failure around that
optional value. `.Ok(nil)` succeeds without a setting, `.Ok(number)` succeeds
with the parsed number, and `.Err(err)` reports failure. The named result `port`
supplies the type for these constructors.

| Input | Gon result | Display |
| --- | --- | --- |
| `"443"` | `.Ok(443)` — present setting | `"443"` |
| `""` | `.Ok(nil)` — absent setting | `"8080"` |
| `"0"` | `.Ok(0)` — present zero | `"0"` |
| `"abc"` | `.Err(err)` — parsing failed | `"invalid port"` |

`or` handles failure; `??` defaults only on absence. Result is a choice for
an API: existing `(value, error)` functions still work, including useful partial
results alongside errors.

`T?` is the native optional type. Match presence with `case value?` and absence
with `case nil`. A typed nil can be present; nested optionals retain their
individual layers. The language has no predeclared `Option`, `Some` or `None`.

[More examples and boundary cases →](doc/gon-examples.md) ·
[Contextual construction and optional types →](doc/gon-examples.md#contextual-construction-and-optional-types)

## Try Gon

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

3. **Select Gon in your editor.** In VS Code, install the Gon extension's local
   VSIX, disable the official Go extension **for this workspace**, and reload.
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
   and lambdas. Introduce optional values, Result and enums where your APIs need
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

**In development for Gon 2.27, the first stable release.** All nine native
feature cores are implemented. See the [validation record](misc/gon/VALIDATION.md)
for executed checks and platforms, and the [supported tooling limits](misc/gon/features.json).

Gon 2.27 preserves existing **Go 1.27+** source and behavior, with one accepted
exception: prefix `!` cannot be split from its operand by a newline. Write
`!disabled` on one line. New syntax requires Gon; ordinary error returns, nil,
interfaces and Go switches retain their semantics. Go and Gon tools coexist
without replacing or globally shadowing one another.

## Performance, measured

The [24-workload benchmark suite](misc/gon/benchmarks/README.md) compares
unmodified Go 1.27.1, Gon running legacy Go, and Gon running equivalent modern
code with native optionals on Apple M4 / darwin/arm64. Compiler optimizations
remove redundant presence loads, intermediate tags and unused register values.
Runtime costs depend on the workload; no blanket zero-overhead claim is made.

All three variants pass the same 16 correctness tests with identical observable
output. The current report includes every workload, heap allocations, storage
sizes and build costs, including increases. Go/Gon base revisions differ;
modern versus legacy Gon isolates syntax on the same toolchain.
[Full measurements, raw samples and limits →](misc/gon/benchmarks/results/20261004T013816Z/report.md)

## Upstream and license

Gon is a native fork of [Go](https://go.dev/), with upstream copyright notices
and [license](LICENSE) preserved.
[Source and tooling provenance](misc/gon/UPSTREAM.json).
