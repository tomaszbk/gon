# Feature integration gate

Use Gon's toolchain and the maintained modules in this repository. Start every
feature with this inventory; do not discover its scope by searching for the last
feature's node name. Supported validation architectures are amd64, arm64,
riscv64 and wasm. Record which platforms actually ran; a local pass does not
assert execution on the others.

The compatibility floor is Go 1.27+ (user decision, 2026-10-02). Current
validation uses an unmodified stable Go 1.27.1 baseline. Record the actual
baseline version; keep module language directives and record current evidence accurately.

## Commands

```sh
GON_BASELINE_GO=/path/to/unmodified/go python3 misc/gon/validate.py tooling
GON_BASELINE_GO=/path/to/unmodified/go python3 misc/gon/validate.py errorhandling
GON_BASELINE_GO=/path/to/unmodified/go python3 misc/gon/validate.py conditional
GON_BASELINE_GO=/path/to/unmodified/go python3 misc/gon/validate.py lambda
GON_BASELINE_GO=/path/to/unmodified/go python3 misc/gon/validate.py nullsafety
GON_BASELINE_GO=/path/to/unmodified/go python3 misc/gon/validate.py modern
python3 misc/gon/validate.py modern --list
```

`--only CHECK` (repeatable) runs a subset. Exit 0 means all gates passed with no
pending integration items; 1 means a check failed; 2 means an incomplete feature,
a partial selection, or a usage error. Per-check logs (Go tests use JSON events) and `summary.json` are in
`pkg/gon-validation/<profile>/`. The summary records executed and skipped tests;
a test command matching zero tests fails the gate. Setup failures leave dependent
checks explicitly unexecuted. `GON_BASELINE_GO` is the public baseline setting;
the runner supplies older test-specific aliases for compatibility. It requires
the baseline even for selected checks, so executable pairs cannot silently skip
it. The tooling profile validates infrastructure and existing executable pairs;
it does not declare unfinished language features complete.

The runner invokes only named packages, test filters and individual testdir
features. Never replace it with all.bash, run.bash, dist test, test std,
test cmd/... or the entire test directory. Each profile reports any open items
in `features.json`. Add checks before clearing those items.

## Checklist for each feature

Record a test/check ID or an explicit reason that a row does not apply.

| Area | Source of truth | Required evidence |
| --- | --- | --- |
| Decisions and compatibility | `design/<feature>/README.md` | Accepted rules; old valid Go unchanged; invalid contexts specified |
| Compiler syntax | `src/cmd/compile/internal/syntax` | Tokens, precedence, semicolons, positions, parser/printer/walker, malformed input |
| Public syntax | `src/go/{token,scanner,ast,parser,printer,format}`, `src/cmd/gofmt`, `api/fork.txt` | Parse/format round trips, same legacy formatting, API inventory |
| Type checking | `src/cmd/compile/internal/types2`, generated `src/go/types` | Valid/invalid tests; target types, scopes, inference, `types.Info`; generation consistency |
| Lowering | `src/cmd/compile/internal/{noder,midway}` | Single evaluation, lazy paths, function boundaries, defer; export/import and inlining |
| Executable contract | `test/<feature>.go`, `test/<feature>.dir` | Legacy on baseline and Gon; modern on Gon; equivalent effects/results; invalid programs |
| Structural tooling | `tools/x-tools/go/ast/{astutil,inspector,edge}` | Traversal, filters, cursor edges and replacements reach every new node |
| Semantic tooling | `tools/x-tools/go/{cfg,ssa}`, `refactor/satisfy`, `tools/staticcheck/go/ir` | Explicit flow/type semantics; executable SSA pairs and IR sanity checks |
| Analyzers | `tools/gonpls/internal/settings/gon_analysis_test.go` | Registry-driven corpus, including optional analyzers; diagnostic assertions for affected checks |
| Effects and transformations | x/tools inliner/astutil; Staticcheck code/astutil | Unknown syntax cannot be assumed pure/equal/copyable; valid edits or explicit refusal |
| cgo | `src/cmd/cgo`, `internal/test<feature>` | C call rewriting, paired execution, invalid input, bootstrap adapters |
| Coverage | `src/cmd/cover` | Paired profiles and legacy instrumentation unchanged |
| Editor and CLI | `tools/gonpls`, `misc/gon/test*.py` | Tokens, completion, hover, references, queries and code actions; incomplete source |
| Distribution | `misc/gon/{build,vendor}.py`, module files | Clean reproducible build; vendor drift check; upstream Go isolation |
| Closure | spec, project skill, validation record, `features.json` | Exact commands/results and unresolved limits; no declaration of completion from compile-only evidence |

## Structural traversal and semantic boundaries

`ast.Children(node)` enumerates immediate children using the same inventory as
`ast.Walk`. It has no reflection or semantic backlinks and does not modify
`ast.Node`. Update that inventory once per node. Existing Walk callbacks,
including their final nil visits, retain their behavior.

Use this API for structural descent. It does not describe evaluation order,
short-circuiting, scope, or the distinction between constructing and invoking a
closure. Those need explicit semantics. Inspector's fast type masks and cursor
edge metadata, and astutil's mutable field slots, still require explicit node
registration; do not replace them with an unlabelled traversal that loses fields
or silently misses nodes. Their tests and the checklist make these obligations
visible.

For operations with a conservative contract, unknown syntax means "may have
effects", "not proven equal" or "cannot transform". Do not suppress internal
invariants wholesale or recover panics and report success. The analyzer corpus
uses the real `settings.AllAnalyzers` registry and derives new-node obligations
from `api/fork.txt`. It contains legacy/modern executable fixtures plus targeted
analysis triggers; extend triggers when adding syntax. It is a regression gate,
not proof that every possible analyzer path has been exercised.

## Maintained dependencies

Edit `tools/x-tools`, `tools/staticcheck` and `tools/gonpls` directly. Their
`UPSTREAM.json` files identify imported baselines. Retain upstream module paths,
licenses and tests. Both cmd and gonpls select the same x/tools module via local
replacements. Standard-library export readers use that maintained module too.
`src/vendor` and `src/cmd/vendor` are generated:

```sh
python3 misc/gon/vendor.py
python3 misc/gon/vendor.py --check
```

Do not edit those generated copies or regenerate patches. The vendor check builds
fresh temporary trees from selected module versions and local x/tools, comparing
all files, including modules.txt. Tests run from the maintained source module,
not a second copy in vendor. Other standard-library dependency versions remain
selected by `src/go.mod`; only x/tools is replaced with its maintained source.

When updating upstream, import changes into the maintained modules, carry
forward Gon adaptations, update provenance and go.mod/go.sum, regenerate vendor,
and run the tooling gate. A module version label alone does not attest to Gon
changes; Git history records the maintained sources.

## Current Gon 2.27 status

[STATUS.md](STATUS.md) describes the current implementation;
[VALIDATION.md](VALIDATION.md) records commands, results and platform scope.
All nine native cores and maintained tooling are implemented. Native optional
values use `T?`, direct payloads, nil absence and presence patterns `P?`.
No public Option/Some/None API remains. Result retains qualified and contextual
Ok/Err constructors. See [OPTIONALS.md](OPTIONALS.md) for the current contract
and assisted semantic migration.

Keep compiler/frontends, metadata readers, analyzer registries and editor
services in the same validation scope. Conservative source-inliner/extraction
limits and private typed storage are recorded in features.json. A focused gate
pass does not establish a release pass on every supported platform. Record
executed and cross-compiled targets separately. Update current documentation
in place; do not add handovers or superseded status narratives.
