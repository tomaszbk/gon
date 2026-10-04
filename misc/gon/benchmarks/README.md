# Go / Gon benchmarks

Run from the repository root after building the public Gon tools:

```sh
GON_BASELINE_GO=/absolute/path/to/unmodified/go python3 misc/gon/benchmark.py
```

This compares three standalone programs:

1. Unmodified Go compiling the legacy implementation.
2. Gon compiling exactly the same legacy source.
3. Gon compiling the equivalent implementation with new syntax.

The runner copies `fixtures/common.go`, `common_test.go` and one of
`legacy.go` / `modern.go` into isolated modules. The fixtures directory itself
is not a buildable package: the implementations are alternatives, and the
runner supplies `nonce.go` with the build measurement constant.

The default run takes ten runtime samples of 300 ms per benchmark and seven
build samples per variant. Use `--help` for overrides. All sixteen correctness
tests must pass in every variant, and the three executables must print the
same checksum, failure count and effects. The runner also verifies that the
legacy source files are byte-identical across toolchains.

The modern fixture uses native optional values: `T?`, nil absence, direct
payload assignment, one-layer conversions and presence patterns. Result uses
contextual `.Ok/.Err` when an expected type is known. Benchmark names containing
"Option" identify optional workloads; they do not name a language API.

The twenty-four benchmarks cover all nine additions:

| Feature | Workloads |
| --- | --- |
| Go error propagation and handlers | successful/failed propagation; wrapping handlers |
| Conditional expressions | mixed lazy branches |
| Lambdas | captured callback and combined pipelines |
| Nil safety | present/absent fields, guarded calls, coalescing assignment |
| Enums | construction of unit/positional/record variants; copied storage |
| Matching | exhaustive arms; guards with observable side effects and lazy bodies |
| Option | present/absent propagation/defaults; assignment into mixed slots |
| Result | successful/failed propagation, including Err(nil); local handlers |
| Named arguments | reordered arguments after observable callee evaluation; final slice variadic |
| Combined existing syntax | successful/failed/mixed parse/transform pipelines |

All reported operations process **64 elements**, including bytes and
allocations per operation. Shared counters make lazy evaluation observable;
they also add instrumentation cost equally to the implementations. Timed
workloads contain no I/O. Datasets and string inputs are prepared before timing.
Counters are int64 and the written-order trace is bounded; workload arithmetic
uses bounded input values and batch-local totals. Global sinks and retained
enum storage keep results observable. Constructor/copy workloads include stores
to a reused array; matching reads a prepared array rather than reconstructing it.

The legacy equivalents of enums/Option/Result use explicit tagged structs,
including a separate failure tag that preserves an error payload of nil.
Correctness checks cover present zero, present nil, nested absence, Err(nil), zero
Result, exact once evaluation, guards and copying. They do not assume a Go
`(value, error)` tuple is equivalent to Result. Shared drivers/tests stay identical
across all three variants.

Before collecting timing samples, check fixtures without benchmark execution:

```sh
GON_BASELINE_GO=/absolute/path/to/unmodified/go python3 misc/gon/benchmark.py --correctness-only
```

This retains `correctness.json`, sources, binaries, logs and representation
observations. It does not run benchmark warmups, runtime timing or timed build
samples. A useful shorter repeated run is:

```sh
GON_BASELINE_GO=/absolute/path/to/unmodified/go python3 misc/gon/benchmark.py \
  --samples 7 --benchtime 200ms --build-samples 7
```

With 24 benchmarks and three variants, this requests about 101 seconds of
runtime sampling alone; calibration, warmups, cache preparation and builds add
time. The default requests about 216 seconds of runtime sampling alone.

Each measured run retains `report.md`, `report.html`, `summary.json`, raw samples, command logs,
source copies and binaries in a new directory under `pkg/gon-benchmarks/`.
These outputs are ignored by Git. The JSON includes compiler hashes, toolchain
versions, repository HEAD, integrated upstream provenance, machine details,
environment settings and SHA-256 of every fixture. `report.md` contains every
workload, its three medians, paired modern/Gon-legacy ratios and intervals,
allocation counts, build data and representation measurements; no workload is
discarded for showing a regression. The runner never changes compiler source,
language semantics or compiler optimizations. A run fails if its compiler binary
or source fixtures change during measurement, preserving logs but refusing to
publish those samples as valid.
It requires a released Go 1.27+ baseline, checks a distinct GOROOT and rejects
an executable that accepts Gon's `capabilities --json` command, including a Gon
launcher from a different installation.

`representation.json` records value size for record and unit enums, native optionals with
int/pointer/zero-sized payloads, and Result with error/string payloads. It also
records one separate, untimed retained-pointer sample per variant: 10,000 present
values with distinct pointers, a forced GC, checked checksum, exact slice storage
bytes, observed heap delta and GC cycles. The heap observation includes allocator
and runtime noise; it is not a statistical memory estimate or an acceptance
threshold. Value sizes are host-specific and do not promise a stable enum ABI.

Runtime benchmarks use precompiled test binaries, default optimizations,
`GOMAXPROCS=1`, `GOGC=100`, `GOMEMLIMIT=off`, and no cgo. Each variant is warmed
before measurement. Variants run serially in randomized order within each
round. Medians and observed ranges are retained; the descriptive bootstrap
intervals use paired round ratios. The reported percentage is the median of
those ratios, not the ratio of the displayed ns/op medians. Small changes require
more evidence than one machine/session, especially without CPU affinity or
thermal control.

Build measurements use warmed dependency caches, `-p=1`, `-trimpath`, and an
observable constant change to force recompilation and linking of the main
package. Separate no-change builds measure the cached path. The runtime,
compiler toolchain and dependency cache are not rebuilt from scratch in these
measurements. Wall time includes the public command/launcher. RSS is the
maximum reported by `/usr/bin/time`, not summed concurrent memory across the
process tree. Binary size includes runtime and debug metadata; no stripping.

Use an upstream Go from the same revision as Gon's integrated base if the
goal is to isolate the fork's overhead. When versions differ, Go versus Gon
also includes upstream compiler and standard-library differences. Comparing
the two Gon variants isolates the effect of source syntax on this workload
with the same compiler/runtime. These benchmarks do not establish performance
for other programs or architectures.

The [latest report](results/20261004T013816Z/report.md) measures all 24 workloads
using native optional syntax and the optimized compiler on Apple M4,
macOS 27.0.1, darwin/arm64. It uses ten 300 ms runtime samples and seven build
samples. Go is the released 1.27.1 baseline; Gon's go1.28-devel version records
provenance. All sixteen correctness tests pass in each variant with identical
observable output. All workload results are retained, including increases.

Sources, runner, commands, raw samples, hashes, representation observations and
current validation logs are retained beside the report. Binaries and build
caches remain ignored under pkg/. Native optional int/pointer shapes have the
same measured size as their tagged Go equivalents; zero-sized optional payloads
and unit enums have measured storage overhead. No stable layout ABI is promised.

Only the current published experiment is kept. Superseded report directories
and compiler snapshots are removed. Its diagnostics contain current complete
focused gates, source-only bootstrap and the actually executed/cross-compiled
platform evidence; compare Go versus Gon using this experiment's complete table.
