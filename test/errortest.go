// run

//go:build !js && !wasip1 && gc

// Postfix ! has one meaning in every file: return error from the nearest
// error-last function. Tests use explicit block handlers for Fatal. Legacy and
// modern versions execute on Gon; the legacy also executes on the baseline.
// Failure report positions are compared using MARK comments.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

func main() {
	tool := filepath.Join(runtime.GOROOT(), "bin", "go")
	fixtures := filepath.Join(runtime.GOROOT(), "test", "errortest.dir")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	dir, err := os.MkdirTemp("", "gon-errortest-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			panic(err)
		}
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(fixtures, name))
		if err != nil {
			panic(err)
		}
		return data
	}
	write("go.mod", []byte("module errortest\n\ngo 1.27\n"))
	write("common.go", read("common.go"))

	// results[variant] holds the normalized output of each invocation.
	type variant struct {
		name, file, tool string
		flags            []string
	}
	variants := []variant{
		{"baseline legacy", "legacy.go", baseline, nil},
		{"legacy", "legacy.go", tool, nil},
		{"modern", "modern.go", tool, nil},
		{"modern without inlining", "modern.go", tool, []string{"-gcflags=-l"}},
	}
	invocations := [][]string{
		{"test", "-count=1", "-vet=off", "./..."},
		{"test", "-count=1", "-vet=off", "-run=^$", "-bench=.", "-benchtime=1x", "./..."},
	}
	var want [][]byte
	for _, v := range variants {
		source := read(v.file)
		write("x_test.go", source)
		markers := markerLines(source)
		for i, args := range invocations {
			args = append(append([]string{args[0]}, v.flags...), args[1:]...)
			cmd := exec.Command(v.tool, args...)
			cmd.Dir = dir
			for _, e := range os.Environ() {
				if v.tool == baseline && (strings.HasPrefix(e, "GOROOT=") || strings.HasPrefix(e, "GOTOOLDIR=")) {
					continue
				}
				if !strings.HasPrefix(e, "GOFLAGS=") {
					cmd.Env = append(cmd.Env, e)
				}
			}
			cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off")
			// The harness runs on the host while these test binaries execute on
			// the requested target, through binfmt or the wasm wrapper.
			if goos := os.Getenv("GON_PAIR_GOOS"); goos != "" {
				wrapperRoot := runtime.GOROOT()
				if v.tool == baseline {
					env := exec.Command(v.tool, "env", "GOROOT")
					env.Env = append([]string{}, cmd.Env...)
					root, err := env.Output()
					if err != nil {
						panic(err)
					}
					wrapperRoot = strings.TrimSpace(string(root))
				}
				cmd.Env = append(cmd.Env, "GOOS="+goos, "GOARCH="+os.Getenv("GON_PAIR_GOARCH"), "CGO_ENABLED=0", "PATH="+filepath.Join(wrapperRoot, "lib", "wasm")+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			out, err := cmd.CombinedOutput()
			if err == nil {
				panic(fmt.Sprintf("%s: go %v passed, but its tests fail on purpose:\n%s", v.name, args, out))
			}
			got := normalize(v.name, out, markers)
			if len(want) <= i {
				want = append(want, got)
				checkContents(i, got)
			} else if !bytes.Equal(want[i], got) {
				panic(fmt.Sprintf("%s: go %v differs from the first variant\nwant:\n%s\ngot:\n%s", v.name, args, want[i], got))
			}
		}
	}

	// vet must accept the modern tests, whose failure paths it models too.
	write("x_test.go", read("modern.go"))
	cmd := exec.Command(tool, "vet", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("go vet rejected the modern tests: %v\n%s", err, out))
	}
	checkInvalid(tool)

}

var (
	markPattern  = regexp.MustCompile(`// MARK:(\w+)`)
	linePattern  = regexp.MustCompile(`\bx_test\.go:(\d+):`)
	timePattern  = regexp.MustCompile(`\((\d+\.\d+|\d+)s\)`)
	benchPattern = regexp.MustCompile(`^Benchmark\S+\s+\d+\s+[\d.]+ ns/op`)
)

func markerLines(source []byte) map[string]string {
	markers := make(map[string]string)
	for i, line := range strings.Split(string(source), "\n") {
		if m := markPattern.FindStringSubmatch(line); m != nil {
			markers[fmt.Sprint(i+1)] = m[1]
		}
	}
	return markers
}

// normalize removes timings and replaces every reported line number by the
// name of the marker on that line. A line without a marker is a bug: Fatal
// must report the line of the explicit Fatal handler.
func normalize(name string, out []byte, markers map[string]string) []byte {
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case benchPattern.MatchString(line),
			strings.HasPrefix(line, "goos:"), strings.HasPrefix(line, "goarch:"),
			strings.HasPrefix(line, "pkg:"), strings.HasPrefix(line, "cpu:"):
			continue
		case strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "ok  \t"):
			line = strings.Fields(line)[0] + " " + strings.Fields(line)[1]
		}
		line = timePattern.ReplaceAllString(line, "(Ns)")
		line = linePattern.ReplaceAllStringFunc(line, func(m string) string {
			number := linePattern.FindStringSubmatch(m)[1]
			marker, ok := markers[number]
			if !ok {
				panic(fmt.Sprintf("%s: a failure was reported at x_test.go:%s, which has no MARK comment:\n%s", name, number, out))
			}
			return "x_test.go:@" + marker + ":"
		})
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n"))
}

// checkContents verifies that the failing scenarios really reported failures,
// and that no scenario continued after its failure.
func checkContents(invocation int, got []byte) {
	text := string(got)
	var wantMarkers []string
	if invocation == 0 {
		wantMarkers = []string{"value", "cleanup", "deferred", "only", "multiple", "multiline", "helper", "named",
			"subtest", "parent", "lambda", "method", "tb", "fuzz"}
	} else {
		wantMarkers = []string{"benchmark", "tb"}
	}
	var missing []string
	for _, m := range wantMarkers {
		if !strings.Contains(text, "x_test.go:@"+m+":") {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		panic(fmt.Sprintf("the output does not report markers %v:\n%s", missing, got))
	}
	if strings.Contains(text, "@unreachable") || strings.Contains(text, "@never") {
		panic(fmt.Sprintf("a test continued after a failure or reported a success as a failure:\n%s", got))
	}
}

func checkInvalid(tool string) {
	dir, err := os.MkdirTemp("", "gon-test-propagation-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module invalidtest\n\ngo 1.27\n"), 0600); err != nil {
		panic(err)
	}
	const prefix = `package invalidtest
import "testing"
func only() error { return nil }
type Alias = testing.T
type Suite struct{}
`
	for _, body := range []string{
		`func TestTuple(t *testing.T) { only()! }`,
		`func BenchmarkTuple(b *testing.B) { only()! }`,
		`func FuzzTuple(f *testing.F) { only()! }`,
		`func helper(tb testing.TB) { only()! }`,
		`func helper(t *Alias) { only()! }`,
		`func (Suite) helper(t *testing.T) { only()! }`,
		`func helper(t *testing.T) { only() or err => err }`,
		`func helper(t *testing.T) error { func(){ only()! }(); return nil }`,
		`func helper(t *testing.T) error { var f func(*testing.T) = (u) => { only()! }; _=f; return nil }`,
		`func helper(t *testing.T) { t.Run("sub",(u) => { only() or err => err }) }`,
	} {
		if err = os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(prefix+body), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "test", "-run=^$", "-vet=off", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
		out, err := cmd.CombinedOutput()
		if err == nil || !bytes.Contains(out, []byte("error propagation requires an enclosing function with a final result of type error")) {
			panic(fmt.Sprintf("invalid test propagation accepted or wrong rejection: %s\n%s", body, out))
		}
		if bytes.Contains(out, []byte("internal compiler error")) || bytes.Contains(out, []byte("panic:")) {
			panic(string(out))
		}
	}
}
