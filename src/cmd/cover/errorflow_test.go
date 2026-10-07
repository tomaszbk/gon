package main_test

import (
	"bytes"
	"flag"
	"fmt"
	"internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Coverage of Gon's error-handling constructs: postfix "!" propagation and
// "call() or err { ... }" handlers.
//
// Both constructs can return from the enclosing function in the middle of a
// statement, so the statements after them form a new basic block. If cover
// counted them together with the statements before, code that never ran after
// an early error return would be reported as covered.

const errorFlowDir = "testdata/errorflow"

var update = flag.Bool("update", false, "rewrite the golden files of TestLegacyInstrumentationUnchanged")

// errorFlowPaths are the tests in testdata/errorflow/flow_test.go that are run
// one at a time, each in a fresh process, so that every path has its own
// profile. The order matches the columns of the tables below.
var errorFlowPaths = []string{"TestPathOK", "TestPathBadFirst", "TestPathBadSecond"}

// errorFlowTags lists how often the statement on each tagged line
// ("// @tag" at the end of a line of legacy.go and modern.go) is executed on
// each of errorFlowPaths. Both implementations of the scenario must agree on
// these counts: the statements after a failing "!" or "or err" must be
// reported as not executed, exactly like after an explicit "if err != nil
// { return }".
var errorFlowTags = map[string][3]int{
	"sum-a":      {1, 1, 1},
	"sum-b":      {1, 0, 1},
	"sum-total":  {1, 0, 0},
	"sum-return": {1, 0, 0},

	"scale-parse":   {1, 1, 0},
	"scale-handler": {0, 1, 0},
	"scale-mul":     {1, 0, 0},
	"scale-return":  {1, 0, 0},

	"note-check":   {1, 1, 0},
	"note-handler": {0, 1, 0},
	"note-after":   {1, 1, 0},
	"note-return":  {1, 1, 0},

	"classify-parse":  {1, 1, 1},
	"classify-neg":    {1, 0, 0},
	"classify-nonneg": {0, 0, 1},

	"total-init":   {1, 1, 1},
	"total-loop":   {1, 1, 1},
	"total-parse":  {2, 1, 2},
	"total-add":    {2, 0, 1},
	"total-return": {1, 0, 0},

	"dispatch-double":         {1, 1, 0},
	"dispatch-double-return":  {1, 0, 0},
	"dispatch-negate":         {1, 1, 0},
	"dispatch-negate-handler": {0, 1, 0},
	"dispatch-negate-return":  {1, 0, 0},
	"dispatch-unknown":        {1, 0, 0},

	"apply-closure":        {1, 1, 0},
	"apply-closure-parse":  {1, 1, 0},
	"apply-closure-return": {1, 0, 0},
	"apply-call":           {1, 1, 0},
	"apply-fail":           {0, 1, 0},
	"apply-return":         {1, 0, 0},
}

// errorFlowLegacyTags are the statements that only exist in the legacy
// version, which spells out the checks and returns that "!" implies.
var errorFlowLegacyTags = map[string][3]int{
	"sum-a-fail":           {0, 1, 0},
	"sum-b-fail":           {0, 0, 1},
	"classify-fail":        {0, 1, 0},
	"classify-test":        {1, 0, 1},
	"total-fail":           {0, 1, 1},
	"dispatch-double-fail": {0, 1, 0},
	"apply-closure-fail":   {0, 1, 0},
}

// errorFlowStatements is the number of statements executed and the total
// number of statements that the profile reports for flow.go on each path.
// The two versions differ because the legacy one has the explicit if
// statements and returns.
var errorFlowStatements = map[string][3][2]int{
	"legacy": {{38, 49}, {32, 49}, {15, 49}},
	"modern": {{29, 34}, {20, 34}, {8, 34}},
}

// errorFlowMain is the program built with go build -cover; it makes the same
// calls as TestPathBadFirst.
const errorFlowMain = `package main

import "flowmain/flow"

func main() {
	var notes []string
	flow.Sum("x", "3")
	flow.Scale("x", 3)
	flow.Note(true, &notes)
	flow.Classify("x")
	flow.Total([]string{"x", "2"})
	flow.Dispatch("double", "x")
	flow.Dispatch("negate", "x")
	flow.Apply("x")
}
`

var errorFlowModes = []string{"set", "count", "atomic"}

// errorFlowToolchain is a go command to build the scenario with.
type errorFlowToolchain struct {
	name     string
	variant  string   // "legacy" or "modern": which implementation to build
	goTool   string   // go command
	env      []string // environment, nil for the current one
	toolexec string   // -toolexec argument that substitutes this build of cmd/cover, if any
}

func errorFlowToolchains(t *testing.T) []errorFlowToolchain {
	goTool := testenv.GoToolPath(t)
	toolexec := "-toolexec=" + testcover(t)
	tcs := []errorFlowToolchain{
		{name: "legacy", variant: "legacy", goTool: goTool, toolexec: toolexec},
		{name: "modern", variant: "modern", goTool: goTool, toolexec: toolexec},
	}
	if baseline := coverageBaseline(); baseline != "" {
		// An unmodified toolchain, with its own compiler and cover tool,
		// runs the legacy implementation and must agree with this fork.
		var env []string
		for _, kv := range os.Environ() {
			switch {
			case strings.HasPrefix(kv, "GOROOT="), strings.HasPrefix(kv, "GOTOOLDIR="),
				strings.HasPrefix(kv, "GOFLAGS="), strings.HasPrefix(kv, "GOENV="),
				strings.HasPrefix(kv, "GOTOOLCHAIN="), strings.HasPrefix(kv, "CMDCOVER_"):
			default:
				env = append(env, kv)
			}
		}
		env = append(env, "GOENV=off", "GOTOOLCHAIN=local", "GOFLAGS=")
		tcs = append(tcs, errorFlowToolchain{name: "baseline", variant: "legacy", goTool: baseline, env: env})
	}
	return tcs
}

func coverageBaseline() string {
	return os.Getenv("GON_BASELINE_GO")
}

// command returns a command running the toolchain's go command in dir.
// Builds use this build of cmd/cover, if the toolchain has one to substitute.
func (tc errorFlowToolchain) command(t *testing.T, dir string, args ...string) *exec.Cmd {
	if tc.toolexec != "" && (args[0] == "build" || args[0] == "test") {
		args = slices.Insert(args, 1, tc.toolexec)
	}
	cmd := testenv.Command(t, tc.goTool, args...)
	cmd.Dir = dir
	if tc.env != nil {
		cmd.Env = slices.Clone(tc.env)
	} else {
		cmd.Env = append(cmd.Environ(), "CMDCOVER_TOOLEXEC=true")
	}
	return cmd
}

// TestErrorFlowCoverage compares the legacy and the modern spelling of one
// scenario: the same tests run against both and their coverage profiles must
// report the same outcome for every statement, in all cover modes. The legacy
// version is also run with the unmodified toolchain named by
// GON_BASELINE_GO, if set.
func TestErrorFlowCoverage(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveExec(t)
	t.Parallel()

	if coverageBaseline() == "" {
		t.Log("GON_BASELINE_GO is not set; not running the legacy version with an unmodified toolchain")
	}
	modes := errorFlowModes
	if testing.Short() {
		modes = []string{"count"}
	}
	for _, tc := range errorFlowToolchains(t) {
		for _, mode := range modes {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				t.Parallel()
				testErrorFlowTests(t, tc, mode)
			})
			t.Run(tc.name+"/"+mode+"/build", func(t *testing.T) {
				t.Parallel()
				testErrorFlowBuild(t, tc, mode)
			})
		}
	}
}

// copyErrorFlow writes a module named module into dir, in which the package
// in subdir has the common helpers and the implementation of the variant as
// flow.go, and optionally the tests.
func copyErrorFlow(t *testing.T, dir, module, subdir, variant string, withTests bool) {
	t.Helper()
	pkgDir := filepath.Join(dir, subdir)
	if err := os.MkdirAll(pkgDir, 0777); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"common.go":     "common.go",
		variant + ".go": "flow.go",
	}
	if withTests {
		files["flow_test.go"] = "flow_test.go"
	}
	for from, to := range files {
		data, err := os.ReadFile(filepath.Join(errorFlowDir, from))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, to), data, 0666); err != nil {
			t.Fatal(err)
		}
	}
	gomod := "module " + module + "\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0666); err != nil {
		t.Fatal(err)
	}
}

// testErrorFlowTests builds the test binary once and runs each path on its
// own, checking the profile of each.
func testErrorFlowTests(t *testing.T, tc errorFlowToolchain, mode string) {
	dir := tempDir(t)
	copyErrorFlow(t, dir, "flow", ".", tc.variant, true)

	version := tc.command(t, dir, "version")
	out, err := version.Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %s", tc.name, bytes.TrimSpace(out))

	exe := filepath.Join(dir, "flow.test.exe")
	run(tc.command(t, dir, "test", "-c", "-cover", "-covermode", mode, "-o", exe), t)

	tags := readErrorFlowTags(t, filepath.Join(dir, "flow.go"), tc.variant)
	for i, test := range errorFlowPaths {
		profile := filepath.Join(dir, test+".out")
		cmd := testenv.Command(t, exe, "-test.run=^"+test+"$", "-test.v", "-test.count=1", "-test.coverprofile="+profile)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", test, err, out)
		}
		// Make sure that the test ran and passed, rather than matching nothing.
		if !bytes.Contains(out, []byte("--- PASS: "+test+" ")) {
			t.Fatalf("%s did not run or did not pass:\n%s", test, out)
		}
		checkErrorFlowProfile(t, profile, tc.variant, mode, i, tags)
	}
}

// testErrorFlowBuild checks the coverage data of a program built with
// go build -cover, which goes through cmd/cover in the same way but collects
// the counters in a different way. The program makes the calls of the
// second path.
func testErrorFlowBuild(t *testing.T, tc errorFlowToolchain, mode string) {
	dir := tempDir(t)
	copyErrorFlow(t, dir, "flowmain", "flow", tc.variant, false)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(errorFlowMain), 0666); err != nil {
		t.Fatal(err)
	}

	exe := filepath.Join(dir, "flowmain.exe")
	run(tc.command(t, dir, "build", "-cover", "-covermode", mode, "-o", exe), t)

	covdir := filepath.Join(dir, "covdata")
	if err := os.Mkdir(covdir, 0777); err != nil {
		t.Fatal(err)
	}
	cmd := testenv.Command(t, exe)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covdir)
	run(cmd, t)

	profile := filepath.Join(dir, "profile.txt")
	run(tc.command(t, dir, "tool", "covdata", "textfmt", "-i="+covdir, "-o="+profile), t)

	tags := readErrorFlowTags(t, filepath.Join(dir, "flow", "flow.go"), tc.variant)
	checkErrorFlowProfile(t, profile, tc.variant, mode, 1, tags)
}

// errorFlowTag is the position of the first non-blank character of a tagged
// line, which is where the statement on that line starts.
type errorFlowTag struct {
	line, col int
}

var errorFlowTagRE = regexp.MustCompile(`// @([a-z0-9-]+)\s*$`)

// readErrorFlowTags returns the tagged lines of the implementation in file.
// It also checks that the tags are the ones expected for the variant, so that
// the two implementations can not drift apart.
func readErrorFlowTags(t *testing.T, file, variant string) map[string]errorFlowTag {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	tags := make(map[string]errorFlowTag)
	for i, line := range strings.Split(string(src), "\n") {
		m := errorFlowTagRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if _, dup := tags[m[1]]; dup {
			t.Fatalf("%s: duplicate tag %q", file, m[1])
		}
		tags[m[1]] = errorFlowTag{line: i + 1, col: len(line) - len(strings.TrimLeft(line, " \t")) + 1}
	}
	for tag := range tags {
		_, common := errorFlowTags[tag]
		_, legacyOnly := errorFlowLegacyTags[tag]
		if !common && !(legacyOnly && variant == "legacy") {
			t.Errorf("%s: unexpected tag %q", file, tag)
		}
	}
	for tag := range errorFlowTags {
		if _, ok := tags[tag]; !ok {
			t.Errorf("%s: missing tag %q", file, tag)
		}
	}
	if variant == "legacy" {
		for tag := range errorFlowLegacyTags {
			if _, ok := tags[tag]; !ok {
				t.Errorf("%s: missing tag %q", file, tag)
			}
		}
	}
	return tags
}

// profileBlock is a line of a coverage profile.
type profileBlock struct {
	startLine, startCol int
	endLine, endCol     int
	numStmt, count      int
}

var profileBlockRE = regexp.MustCompile(`^(.*):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$`)

// readProfileBlocks returns the blocks of profile that belong to a file named
// base, sorted by position.
func readProfileBlocks(t *testing.T, profile, base, mode string) []profileBlock {
	t.Helper()
	data, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[0] != "mode: "+mode {
		t.Fatalf("%s: first line is %q, want mode %q", profile, lines[0], mode)
	}
	var blocks []profileBlock
	for _, line := range lines[1:] {
		m := profileBlockRE.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("%s: malformed line %q", profile, line)
		}
		if filepath.Base(m[1]) != base {
			continue
		}
		var n [6]int
		for i := range n {
			n[i], _ = strconv.Atoi(m[i+2])
		}
		blocks = append(blocks, profileBlock{n[0], n[1], n[2], n[3], n[4], n[5]})
	}
	sort.Slice(blocks, func(i, j int) bool {
		a, b := blocks[i], blocks[j]
		if a.startLine != b.startLine {
			return a.startLine < b.startLine
		}
		return a.startCol < b.startCol
	})
	return blocks
}

// checkErrorFlowProfile checks the profile of one of errorFlowPaths.
func checkErrorFlowProfile(t *testing.T, profile, variant, mode string, path int, tags map[string]errorFlowTag) {
	t.Helper()
	blocks := readProfileBlocks(t, profile, "flow.go", mode)
	if len(blocks) == 0 {
		t.Fatalf("%s: no blocks for flow.go", profile)
	}

	// Counter blocks must not overlap: a statement is counted exactly once,
	// in the block that starts it.
	for i := 1; i < len(blocks); i++ {
		a, b := blocks[i-1], blocks[i]
		if a.endLine > b.startLine || a.endLine == b.startLine && a.endCol > b.startCol {
			t.Errorf("overlapping blocks %v and %v", a, b)
		}
	}

	covered, total := 0, 0
	for _, b := range blocks {
		total += b.numStmt
		if b.count > 0 {
			covered += b.numStmt
		}
	}
	if want := errorFlowStatements[variant][path]; covered != want[0] || total != want[1] {
		t.Errorf("%s: %d of %d statements covered, want %d of %d", errorFlowPaths[path], covered, total, want[0], want[1])
	}

	for tag, pos := range tags {
		want := errorFlowTags[tag]
		if variant == "legacy" {
			if w, ok := errorFlowLegacyTags[tag]; ok {
				want = w
			}
		}
		got := -1
		for _, b := range blocks {
			if (b.startLine < pos.line || b.startLine == pos.line && b.startCol <= pos.col) &&
				(pos.line < b.endLine || pos.line == b.endLine && pos.col < b.endCol) {
				got = b.count
				break
			}
		}
		if got < 0 {
			t.Errorf("%s: statement %q at line %d is in no block", errorFlowPaths[path], tag, pos.line)
			continue
		}
		if mode == "set" {
			// A set profile only says whether the block ran.
			if (got > 0) != (want[path] > 0) {
				t.Errorf("%s: statement %q has count %d, want executed=%v", errorFlowPaths[path], tag, got, want[path] > 0)
			}
		} else if got != want[path] {
			t.Errorf("%s: statement %q has count %d, want %d", errorFlowPaths[path], tag, got, want[path])
		}
	}
}

// TestErrorHandlingRanges checks where cover places its counters around
// error propagation and handlers, using the bracket markers of
// TestCommentedOutCodeExclusion: each marked range is a block of its own.
func TestErrorHandlingRanges(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	t.Parallel()

	tests := []struct {
		name string
		src  string
	}{
		{"propagation", `package main

func f(s string) (int, error) {
«	n := atoi(s)!»
«	n *= 2
	return n, nil»
}
`},
		{"propagation in several statements", `package main

func f(a, b string) (int, error) {
«	x := atoi(a)!»
«	y := atoi(b)!»
«	use(x, y)
	return x + y, nil»
}
`},
		{"propagation without result", `package main

func f() error {
«	step()!»
«	step()!»
«	return nil»
}
`},
		{"propagation in an argument", `package main

func f(s string) (int, error) {
«	return atoi(atoi(s)!), nil»
}
`},
		{"handler that returns", `package main

func f(s string) (int, error) {
«	n := atoi(s) or err {»
«		return 0, err»
	}
«	n *= 2
	return n, nil»
}
`},
		{"handler that finishes normally", `package main

func f(fail bool) int {
«	n := 0
	check(fail) or err {»
«		n = 1»
	}
«	n++
	return n»
}
`},
		{"handler with statements", `package main

func f(s string) (int, error) {
«	n := atoi(s) or err {»
«		log(err)
		return 0, err»
	}
«	return n, nil»
}
`},
		{"handlers in a row", `package main

func f(a, b string) (int, error) {
«	x := atoi(a) or err {»
«		return 0, err»
	}
«	y := atoi(b) or err {»
«		return x, err»
	}
«	return x + y, nil»
}
`},
		{"propagation and handler in an if header", `package main

func f(a, b string) (int, error) {
«	if n := atoi(a)!; n > 0 {»
«		return n, nil»
	}
«	if n := atoi(b) or err {»
«		return 0, err»
	}; n > 0 {
«		return n, nil»
	}
«	return 0, nil»
}
`},
		{"propagation and handler in an else if header", `package main

func f(a bool, b string) (int, error) {
«	if a {»
«		return 1, nil»
«	} else if n := atoi(b) or err {»
«		return 0, err»
	}; n > 0 {
«		return n, nil»
	}
«	return 0, nil»
}
`},
		{"propagation and handler in a for header", `package main

func f(a, b string) (int, error) {
«	total := 0
	for i := atoi(a)!; i < 3; i++ {»
«		total += i»
	}
«	for i := atoi(b) or err {»
«		return 0, err»
	}; i < 3; i++ {
«		total += i»
	}
«	return total, nil»
}
`},
		{"propagation and handler in a range expression", `package main

func f(a string) (int, error) {
«	total := 0
	for _, n := range list(a)! {»
«		total += n»
	}
«	for _, n := range list(a) or err {»
«		return 0, err»
	} {
«		total += n»
	}
«	return total, nil»
}
`},
		{"propagation and handler in a switch", `package main

func f(a, b string) (int, error) {
«	switch atoi(a)! {»
	case 1:
«		return 1, nil»
	}
«	switch n := atoi(b) or err {»
«		return 0, err»
	}; n {
	case 1:
«		return 1, nil»
	}
«	return 0, nil»
}
`},
		{"handler in a type switch guard", `package main

func f(a string) (int, error) {
«	switch v := any(atoi(a) or err {»
«		return 0, err»
	}).(type) {
	case int:
«		return v, nil»
	}
«	return 0, nil»
}
`},
		{"cases", `package main

func f(kind, arg string) (int, error) {
«	switch kind {»
	case "a":
«		n := atoi(arg)!»
«		return n, nil»
	case "b":
«		n := atoi(arg) or err {»
«			return 0, err»
		}
«		return -n, nil»
	}
«	return 0, nil»
}
`},
		{"expression over several lines", `package main

func f(a, b string) (int, error) {
«	sum := add(
		atoi(a)!,
		atoi(b) or err {»
«			return 0, err»
		},
	)
«	use(sum)
	return sum, nil»
}
`},
		{"propagation in a function literal", `package main

func f(s string) (int, error) {
«	g := func() error {»
«		step()!»
«		return nil»
	}
«	err := g()
	return 1, err»
}
`},
		{"handler in a function literal", `package main

func f(s string) (int, error) {
«	g := func() (int, error) {»
«		n := atoi(s) or err {»
«			return 0, err»
		}
«		return n, nil»
	}
«	return g()»
}
`},
		{"label", `package main

func f(s string) (int, error) {
«	n := atoi(s)!»
L:
«	n *= 2
	if n < 10 {»
«		goto L»
	}
«	return n, nil»
}
`},
		{"return with handler", `package main

func f(s string) (int, error) {
«	return atoi(s) or err {»
«		return 0, err»
	}, nil
}
`},
		{"case expression", `package main

func f(a string) int {
«	switch {»
	case atoi(a) or err {
«		return 0»
	} > 0:
«		return 1»
	}
«	return 2»
}
`},
		{"defer and go", `package main

func f(s string) error {
«	defer use(atoi(s)!)»
«	go use(atoi(s)!)»
«	return nil»
}
`},
		{"send and inc", `package main

func f(c chan int, s string) error {
«	c <- atoi(s)!»
«	n := 0
	n += atoi(s)!»
«	n++
	return nil»
}
`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src, want := parseBrackets([]byte(test.src))
			got := coverRanges(t, src)
			compareRanges(t, src, got, want)
		})
	}
}

// TestLegacyInstrumentationUnchanged checks that source without error
// handling syntax is instrumented exactly as before, in every mode. The golden
// files were produced by cmd/cover before it knew about the constructs; the
// source uses everything around which the counter placement is delicate, plus
// identifiers and operators that resemble the new syntax.
//
// Run the test with -update to rewrite the golden files, which is only
// correct if the change in instrumentation is intended.
func TestLegacyInstrumentationUnchanged(t *testing.T) {
	testenv.MustHaveExec(t)
	t.Parallel()

	const dir = "testdata/legacyflow"
	for _, mode := range errorFlowModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			cmd := testenv.Command(t, testcover(t), "-mode="+mode, "-var=GoCover", "legacy.go")
			cmd.Dir = dir
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			got, err := cmd.Output()
			if err != nil {
				t.Fatalf("cover failed: %v\n%s", err, &stderr)
			}
			golden := filepath.Join(dir, "legacy."+mode+".golden")
			if *update {
				if err := os.WriteFile(golden, got, 0666); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("instrumentation of legacy.go in mode %s differs from %s:\n%s", mode, golden, firstDifference(got, want))
			}
		})
	}
}

// firstDifference describes where got and want first differ.
func firstDifference(got, want []byte) string {
	g := strings.Split(string(got), "\n")
	w := strings.Split(string(want), "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d:\n got: %s\nwant: %s", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("got %d lines, want %d lines", len(g), len(w))
}
