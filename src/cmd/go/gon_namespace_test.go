package main_test

import (
	"internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The same executable scenario uses an existing Go module's package and the
// new Gon standard package. Check the former with unmodified Go as well.
func TestGonNamespacePair(t *testing.T) {
	testenv.MustHaveGoRun(t)
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		t.Fatal("GON_BASELINE_GO must name an unmodified Go toolchain")
	}
	const program = `package main
import "fmt"
import "gon/seq"
func main() {
	events := ""
	values := seq.Map([]int{1, 2, 3}, func(n int) int { events += fmt.Sprint(n); return 2*n })
	if events != "123" || len(values) != 3 || values[0] != 2 || values[1] != 4 || values[2] != 6 { panic("map effects") }
	fmt.Println(values, events)
}
`
	const legacySeq = `package seq
func Map[T, U any](values []T, f func(T) U) []U {
	result := make([]U, len(values))
	for i, value := range values { result[i] = f(value) }
	return result
}
`
	for _, test := range []struct {
		name, tool string
		legacy     bool
	}{
		{"legacy baseline", baseline, true},
		{"legacy Gon", testGo, true},
		{"modern Gon", testGo, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, source string) {
				t.Helper()
				file := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("go.mod", "module gon\n\ngo 1.27\n")
			write("main.go", program)
			if test.legacy {
				write("seq/seq.go", legacySeq)
			}
			cmd := testenv.Command(t, test.tool, "run", ".")
			cmd.Dir = dir
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "GOROOT=") && !strings.HasPrefix(entry, "GOTOOLDIR=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			if test.tool == testGo {
				cmd.Env = append(cmd.Env, "GOROOT="+testGOROOT)
			}
			cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GO111MODULE=on", "GOWORK=off")
			if out, err := cmd.CombinedOutput(); err != nil || string(out) != "[2 4 6] 123\n" {
				t.Fatalf("run: %v\n%s", err, out)
			}
		})
	}
}
