package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGonCgoErrorHandlingCheck(t *testing.T) {
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		t.Fatal("GON_BASELINE_GO must name an unmodified Go toolchain for the cgo pair")
	}
	const common = `package main
/*
#include <errno.h>
static int divide(int n) {
	if (n == 0) { errno = EDOM; return -1; }
	return 6 / n;
}
*/
import "C"
import "fmt"
func main() {
	for _, call := range []func(int) (int, error){bang, block, context} {
		for _, n := range []int{2, 0} {
			value, err := call(n)
			if (n == 2 && (value != 3 || err != nil)) || (n == 0 && (value != 0 || err == nil)) { panic("cgo errno") }
		}
	}
	fmt.Println("PASS cgo errno")
}
`
	const modern = `
func bang(n int) (int, error) { value := C.divide(C.int(n))!; return int(value), nil }
func block(n int) (int, error) { value := C.divide(C.int(n)) or err { return 0, err }; return int(value), nil }
func context(n int) (int, error) { value := C.divide(C.int(n)) or err => err; return int(value), nil }
`
	const legacy = `
func bang(n int) (int, error) { value, err := C.divide(C.int(n)); if err != nil { return 0, err }; return int(value), nil }
func block(n int) (int, error) { value, err := C.divide(C.int(n)); if err != nil { return 0, err }; return int(value), nil }
func context(n int) (int, error) { value, err := C.divide(C.int(n)); if err != nil { return 0, err }; return int(value), nil }
`
	for _, test := range []struct{ name, tool, body string }{
		{"baseline legacy", baseline, legacy},
		{"gon legacy", filepath.Join(runtime.GOROOT(), "bin", "go"), legacy},
		{"gon modern", filepath.Join(runtime.GOROOT(), "bin", "go"), modern},
	} {
		t.Run(test.name, func(t *testing.T) {
			tree := writeTree(t, "-- go.mod --\nmodule example.com/cgoerrno\n\ngo 1.27\n-- main.go --\n"+common+test.body)
			cmd := exec.Command(test.tool, "run", ".")
			cmd.Dir = tree
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "GOROOT=") && !strings.HasPrefix(entry, "GOTOOLDIR=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOWORK=off")
			if out, err := cmd.CombinedOutput(); err != nil || string(out) != "PASS cgo errno\n" {
				t.Fatalf("run: %v\n%s", err, out)
			}
			if test.name != "baseline legacy" {
				var check gonCheck
				gonJSON(t, tree, nil, &check, "check", ".").checkCode(0)
				if check.Summary.Errors != 0 {
					t.Fatalf("false cgo type error: %+v", check)
				}
			}
		})
	}
}
