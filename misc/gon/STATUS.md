# Current Gon status

Gon 2.27 is the current development target, compatible with Go 1.27+ on the
supported platforms subject to accepted exceptions. Go 1.27.1 is the unmodified
baseline; the fork's go1.28-devel version identifies provenance. Keep `.go`,
`_test.go`, `go.mod` and `go.work`. Select public `gon`/`gonpls` per project,
alongside ordinary Go. See the [adoption guide](../../README.md#use-gon-in-an-existing-go-project).

All nine native cores and maintained tooling are implemented: error propagation,
conditional expressions, lambdas, nil safety, enums, exhaustive matching,
optional values, Result and named arguments. Both frontends/checkers, public
AST/formatting, lowering, metadata readers, cgo/coverage, CFG/SSA, Staticcheck IR
and gonpls are integrated. Maintained modules are `tools/x-tools`,
`tools/staticcheck` and `tools/gonpls`; vendor trees are generated.

Optional values now use the native type `T?`, nil absence, immediate payload
lifting and presence patterns `P?`. Typed nil remains present; nested layers
never flatten. No predeclared Option/Some/None API remains. Result retains
`.Ok/.Err`, qualified constructors/patterns and explicit Go tuple adapters.
The [optional contract](OPTIONALS.md) explains reflection and the reviewable
`gon refactor optionals` migration. Ordinary user names retain Go semantics.

[VALIDATION.md](VALIDATION.md) records the latest commands, complete focused
profile results, bootstrap, editor and target evidence. Executed targets and
cross-compiled targets are distinguished. [features.json](features.json)
records supported limits; [INTEGRATION.md](INTEGRATION.md) defines the maintained
workflow. These checks do not establish a complete release pass on every
supported platform.

Representation uses a discriminator and separate typed storage for GC safety.
Unit and zero-sized shapes have measured overhead. There is no stable storage
ABI or automatic serialization; C and serialization boundaries need explicit
adapters. First-class positional constructors disable compiler inlining.
Source inlining/extraction conservatively decline unsupported named, lazy and
contextual moves. The [current benchmark](benchmarks/README.md) publishes all
24 workloads and their costs. No zero-overhead claim is made.

[UPSTREAM.json](UPSTREAM.json) records integrated Go revision
`67c1d421161d3d1ae9f5fd005e84c29fd0d9f896`. Integration used three-way source
comparison without importing upstream merge ancestry. Use that revision as the
previous pristine source for updates; preserve each tooling module's provenance,
upstream licenses, paths and tests.

Update current records in place. Handovers and superseded published experiments
are removed. Root AGENTS.md and design Markdown remain local and ignored.
Legacy Go fixtures remain required compatibility tests. Gon 2.28 would adopt
Go 1.28; Gon 3.0 remains a future, separately specified language direction.
