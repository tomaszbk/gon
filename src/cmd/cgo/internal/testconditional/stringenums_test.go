package testconditional

import (
	"internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPairedCgoStringEnums(t *testing.T) {
	testenv.MustHaveGoRun(t)
	testenv.MustHaveCGO(t)
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		t.Fatal("GON_BASELINE_GO must name an unmodified Go toolchain")
	}
	var output string
	for _, tc := range []struct {
		variant, tool string
		baseline      bool
	}{
		{"legacy", testenv.GoToolPath(t), false},
		{"modern", testenv.GoToolPath(t), false},
		{"legacy", baseline, true},
	} {
		dir := gonFeatureModule(t, "stringenums", tc.variant)
		source, err := os.ReadFile("testdata/stringenums_common.go")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "scenario.go"), source, 0666); err != nil {
			t.Fatal(err)
		}
		out, stderr, err := goCommand(t, tc.tool, tc.baseline, dir, "run", ".")
		if err != nil {
			t.Fatalf("%s baseline=%v: %v\n%s%s", tc.variant, tc.baseline, err, out, stderr)
		}
		if !strings.HasSuffix(out, "PASS\n") {
			t.Fatalf("scenario did not pass: %s", out)
		}
		if output == "" {
			output = out
		} else if output != out {
			t.Fatalf("unequal behavior: want %q, got %q", output, out)
		}
		out, stderr, err = goCommand(t, tc.tool, tc.baseline, dir, "test", "-vet=off", "-cover", "-coverprofile=coverage.out", "-count=1", ".")
		if err != nil {
			t.Fatalf("%s baseline=%v coverage: %v\n%s%s", tc.variant, tc.baseline, err, out, stderr)
		}
		profile, err := os.ReadFile(filepath.Join(dir, "coverage.out"))
		if err != nil || !strings.Contains(string(profile), "/scenario.go:") || !strings.Contains(string(profile), "/main.go:") {
			t.Fatalf("%s baseline=%v missing scenario or enum wrapper coverage: %v\n%s", tc.variant, tc.baseline, err, profile)
		}
	}
}

func TestCgoStringEnumDiagnostics(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveCGO(t)
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"go.mod": "module invalidstringenum\n\ngo 1.27\n",
		"invalid.go": `package main
// #define invalid_role 42
import "C"
type Role enum string { default Unknown(string); Invalid = C.invalid_role }
func main() {}
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0666); err != nil {
			t.Fatal(err)
		}
	}
	out, stderr, err := goCommand(t, testenv.GoToolPath(t), false, dir, "build", "-o", filepath.Join(dir, "invalid.exe"), ".")
	if err == nil {
		t.Fatal("non-string C constant compiled as string enum spelling")
	}
	out += stderr
	if !strings.Contains(out, "string enum spelling must be a constant string") {
		t.Fatalf("missing spelling diagnostic:\n%s", out)
	}
	for _, crash := range []string{"panic:", "goroutine ", "unexpected type", "internal compiler error"} {
		if strings.Contains(out, crash) {
			t.Fatalf("invalid C spelling crashed a tool:\n%s", out)
		}
	}
}
