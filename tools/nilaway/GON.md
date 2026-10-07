# Maintained NilAway integration

This directory preserves Uber's `go.uber.org/nilaway` module, Apache-2.0 license,
NOTICE, upstream source and test corpus. `UPSTREAM.json` records the imported
revision and exact pseudo-version. Gonpls selects this maintained module with a
local replacement; its `golang.org/x/tools` replacement selects the same
maintained source used by Gon's other analyzers.

NilAway is opt-in through the gonpls `nilaway` setting or `gon check --nilaway`.
It reports warnings through the existing diagnostic engine and does not change
the compiler's nil semantics or add non-null types.

Ordinary Go packages use NilAway's original assertion-tree frontend. Packages
using Gon constructs use the maintained x/tools SSA builder and feed the same
annotation triggers, inter-package inference and diagnostics. This preserves
written argument order, short-circuit branches, optional and enum payload
selection, pattern-test bindings, lambda captures and propagation returns.
Typed nil payloads remain potentially nil even after a presence test. A boxed
typed nil remains a nonnil interface, with its payload checked after assertion.
The public gonpls and CLI opt-in enable anonymous-function analysis for ordinary
Go too. Standalone NilAway retains its upstream configuration flag defaults.
Lexical captures and function assignments use type-checker objects, so editor
parsing with `SkipObjectResolution` retains
the same analysis as command-line parsing.

The SSA adapter tracks reaching stores, local map keys and slice elements,
callee writes through pointer arguments, closure reads at invocation, interface
method signatures and concrete receiver calls. Proved optional/enum payload
identities are exported as analyzer facts, preserving the actual argument's
nilness across imported generic identity functions. Unproved external calls and
escaped writes receive conservative nil-flow triggers. Such warnings are not a
proof that a runtime panic occurs, and the absence of warnings is not a general
non-null guarantee. Unexpected analysis errors are emitted as diagnostics while
retaining assertions already collected.

The migrated `llm_teacher` application retains conservative warnings for external
constructor and framework contracts. These include embedded directory entries,
MCP tool elements, request URLs, injected services, HTTP response bodies and
NilAway's imported successful-response facts for `net/http.Client.do`. The app's
retained validation JSON distinguishes those warnings from resolved source-flow
issues. Nil propagation preserves partial tuples: a successful `(nil, nil)`
return can still cause a real nil dereference and is covered by a positive test.

Generated error-tree search helpers are modeled through their source subjects
and payloads. Fatal/FailNow/Skip termination excludes the unreachable zero return
generated for test propagation. Ordinary pointer-field nil guards apply to both
whole-struct and direct-field loads, and trusted error-return hooks retain the
upstream protocol for failure-only partial results.

Gon adaptations outside the SSA adapter register native enum/optional annotation
sites, preserve partial results when a subanalysis reports an error, use valid
source positions for internal diagnostics, and handle function literal expression
nodes exposed by the maintained CFG builder. All upstream tests remain present.

`nilaway_gon_test.go` compares safe and genuinely unsafe Gon constructs with
ordinary Go counterparts. Fixtures cover the accepted features, including named
arguments, interpolation, `gon/seq`, optional propagation and test-file Fatal
handling. `nilaway_gon_memory_test.go` checks aliases, post-capture writes,
missing map keys, helper parameters, interface calls, method receivers and
imported generic optional identities. `test/nilanalysis.go` executes paired
legacy/modern scenarios with the unmodified baseline and Gon, including recovered
actual nil panics and `_test.go` failure handling.

Run the maintained corpus with `gon test -p=1 -parallel=2 ./... -count=1` from
this module. Bounded concurrency runs every test while limiting the memory used
by independent analysis corpora. The repository's `nilanalysis` validation
profile also checks gonpls opt-in settings,
the semantic CLI, the analyzer registry and the executable pair. Validation uses
the project's selected Gon binaries, not a global replacement of Go tools.
