# Gon tooling commands

`gon` adds these commands to the Go command. Every Go command (`build`, `test`,
`run`, `vet`, `fmt`, `env`, `mod`, `doc`, ...) keeps its upstream behavior.
`gon fix` additionally recognizes safe opportunities to adopt Gon syntax.

| Command | Purpose | Semantic engine |
| --- | --- | --- |
| `gon query def\|refs\|impls\|type <target>...` | Declarations, references, implementations, types | yes |
| `gon query symbols <query>...` | Fuzzy search of workspace symbols | yes |
| `gon refactor rename <target> <name>` | Rename with preview and revision check | yes |
| `gon refactor apply <plan.json\|->` | Apply a previewed plan | no |
| `gon check [packages\|files]` | Parse, type and analysis diagnostics, without building | yes |
| `gon explain <code>...` | Documentation for diagnostic codes and analyzers | no |
| `gon capabilities` | Tooling supported by the selected toolchain | no |

The semantic commands reuse the gonpls engine (Gon's parser, type checker and
analyzers); there is no separate implementation of the language rules. Each
command starts a gonpls session in its own process, reads the **saved files**
and exits, so results never come from stale state or from unsaved editor
buffers. The editor's language server is a separate, long-lived process. Run
`gon help tooling` or `gon help <command>` for flags.

Use the `gon` of the project's selected toolchain. The launcher pins `GOROOT`
and `GOTOOLCHAIN=local` for everything it runs.

## Syntax modernization

```sh
gon fix -diff ./...                 # preview changes; exit 1 when changes exist
gon fix ./...                       # apply suggested fixes
gon check --severity=hint ./...      # list suggestions without changing files
gon fix -gonerrors -diff ./...       # preview only error-handling conversions
gon tool fix help gonerrors          # describe one analyzer
```

The shared `gonerrors`, `gonconditional`, `gonnil`, and `gonlambda` analyzers
also provide editor hints and quick fixes. `gon check --severity=hint --json`
includes their proposed edits, and `gon explain <analyzer>` describes them.
They are suggestions, not new language errors; existing Go source remains valid.
`gon fix` keeps its existing package selection, analyzer flags and diff format,
and still includes the upstream Go modernizers. Its flags and exit status follow
`gon help fix`, independently of the semantic-command schema below.

The recognized patterns include call results followed by a fresh error check
(`!` for equivalent zero-value error returns and an exact `<first param>.Fatal(err)` in a
qualifying test function; otherwise `or err { ... }`),
simple return/assignment branches (conditional expressions), nil defaults and
guards (`??=`, `??`, `?.`, `?(`), and function literals with known contextual
signatures (lambdas).

Conversions are deliberately conservative. They decline cases that would lose
partial results, remove a still-used error binding, alter evaluation or typing,
or discard comments or required imports. They do not automatically convert
introduce `T?`, lift
payloads into optionals, or rewrite enums and matching. These constructs are
implemented, but no automatic modernization analyzer is supplied for them.
The absence of a suggestion does not mean a manual rewrite is impossible.
If nested fixes overlap, the existing fix driver applies compatible edits and
asks for another run;
newly exposed source shapes may also need a manual rewrite.

`T?` is a native optional type. `case value?` extracts presence and `case nil`
matches absence. Typed nil remains present; nested layers never flatten.
Semantic output spells optionals with `T?`, including imported types. Type
queries identify `optional-type`, `optional-propagation` and
`optional-coalescing`. Source inlining and extraction conservatively decline
unsupported lazy/context-sensitive transformations.

## Arguments

Flags may appear before, between or after arguments; `--` ends flag parsing.
Both `-flag` and `--flag`, and `--flag=value` or `--flag value`, are accepted.

A **target** is a position or a symbol:

- `file.go:line:column`: 1-based line and 1-based column counted in UTF-8
  bytes, as in compiler diagnostics. `file.go:#offset` is a 0-based byte offset.
  Positions address anything, including local variables, handler bindings,
  function literals, the `!` or `or` of a Gon error-handling construct, the `?`
  of an optional type, a pattern test or an embedded interpolation expression.
- A symbol: `Name`, `Type.Method` or `Type.Field` in the package of the
  current directory; `pkg.Name` for a package imported there; `./dir.Name`
  (or `../dir`, or an absolute directory); or an import path such as
  `example.com/m/pkg.Type.Method`. A final path element may contain dots
  (`gopkg.in/yaml.v3.Node`); the longest resolvable package path wins.
  `gon query symbols` prints fully qualified names usable as targets.

Several targets may be given to a query. `--limit n` (default 100, 0 means no
limit) and `--offset n` page through the items of each target.

Common semantic flags: `--json` and `--tags a,b` (build tags of the analyzed
configuration). `gon check` also accepts `--severity`
(`error`, `warning` (default), `info`, `hint`), `--category`
(`language`, `analysis`, `all`), `--code`, `--staticcheck`, `--nilaway` and paging flags. NilAway is opt-in and
reports warnings. Enable the same analyzer in VS Code with:

```json
{
  "gon.serverSettings": { "nilaway": true }
}
```

## Exit status

| Status | Meaning |
| --- | --- |
| 0 | Completed; nothing requested failed. Warnings never cause a failure. |
| 1 | Completed with findings: error diagnostics, an unresolved target, a rejected or stale refactoring, an unknown code. |
| 2 | Invalid command line. |
| 3 | Infrastructure failure: the toolchain or workspace could not be used. |

Errors are printed to standard error, or as the JSON `error` object with
`--json`.

## JSON results (schema version 1)

Every JSON result is one object with these common fields:

```json
{
  "schemaVersion": 1,
  "operation": "query.refs",
  "ok": true,
  "toolchain": {"root": "...", "languageServer": ".../gon/bin/gonpls", "gonpls": "...", "goVersion": "..."},
  "configuration": {"workspace": "...", "goos": "darwin", "goarch": "arm64", "goflags": "", "buildFlags": ["-tags=a"], "staticcheck": false},
  "error": {"kind": "not-found", "message": "..."}
}
```

`configuration` is present for semantic commands; `error` only on failure. Error kinds: `usage`, `not-found`, `ambiguous`, `rejected`,
`stale`, `query-failed`, `workspace`, `internal`.

A **location** is `{"path", "line", "column", "endLine", "endColumn",
"offset", "endOffset"}` with absolute paths and the encoding described above.
`revision` maps every file used by a result to the SHA-256 of the exact content
that was analyzed.

**Queries** (`query.def`, `query.refs`, `query.impls`, `query.type`,
`query.symbols`) return `results`, one per argument: `target` (spec, kind,
location, object), or `query` for symbols; `total`; `items` (location, source
`text`, `declaration` for the declaration among references, `object` with name,
kind, package and signature, `hover` documentation for definitions,
`container` for symbols); `truncated` (`omitted`, `nextOffset`) when paged; and
`error` for a target that failed. `query type` returns `type` with the
expression, type, underlying type, mode, constant value and a Gon `construct`
when relevant: `enum-type`, `match-expression`, `pattern-test`,
`error-propagation`, `test-error-propagation`, `error-handler`, `error-context`,
`conditional-expression`, `lambda`, `optional-propagation`, `optional-type`,
`nil-guard`, `option-guard`, `safe-navigation`, `nil-coalescing`,
`optional-coalescing` and `string-interpolation`.

**`check`** returns `patterns`, `packages`, `verified` (`parse`,
`type-check`, `analysis`), `analyzers` (names), `notVerified` (for example
building with the compiler, tests, `gon vet`), `warnings`, `summary` (counts by
severity after filtering), `total`, `diagnostics` and `revision`. A diagnostic
has location, `severity`, `category` (`language` for syntax, type and package
errors; `analysis` for optional analyzers; `load` for packages that could not
be selected), `source`, `code` (accepted by `gon explain`), `message`, source
`text`, `tags`, `related` and mechanical `fixes` (title and edits). Fixes are
never applied by `gon check`. Diagnostics are deduplicated across package
variants.

**`refactor.rename`** returns a plan: `status` (`planned` or `applied`),
`target`, `newName`, `summary` and `files`. Each file has `path`, `sha256`
(analyzed content), `newSha256`, `formatting` and sorted, non-overlapping
`edits` (location and `newText`) relative to the analyzed content.
`refactor.apply` returns the same plan with status `applied`.

**`explain`** returns `explanations` with `code`, `kind` (`type-error` or
`analyzer`), `number`, `summary`, `documentation`, `cases` (message, meaning
and fix for Gon codes), `defaultEnabled`, `url`, `source` and `references`.
Type-checker codes come from the selected toolchain's
`src/internal/types/errors/codes.go`.

**`capabilities`** returns `capabilities` (`queries`, `rename`,
`refactorApply`, `check`, `explain`, `persistent` (always `false`: no state is
kept between commands), `positions`) and `commands`.
Editor integrations use it to detect support before offering a feature.

Within schema version 1, new commands and fields may be added. Removing or
changing the meaning of a field requires a new schema version.

## Refactoring

`gon refactor rename` checks that every analyzed file still has the content
the engine saw, then writes all files or none. With `--dry-run` it prints a
unified diff, or with `--json` a plan that `gon refactor apply` applies later
after the same check. A plan becomes stale when any of its files changes; stale
plans are rejected without writing any file.

Files that were gofmt-clean are formatted after renaming (`formatting: gofmt`
when that realigned code). Files that were not gofmt-clean keep their
formatting (`skipped`), so a rename never introduces unrelated changes.
Renames that would create, move or delete files (package renames) are rejected.

## Performance

A command takes about 0.1–1 s in the workspaces measured (0.9 s for a
references query over the Gon repository's `src` module) once gonpls's on-disk
cache, shared across processes, is populated; the first command after
rebuilding gonpls can take several seconds.
