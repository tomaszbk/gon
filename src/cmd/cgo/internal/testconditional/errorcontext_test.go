package testconditional

import (
	"internal/testenv"
	"os"
	"strings"
	"testing"
)

func TestPairedCgoErrorContext(t *testing.T) {
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
		dir := gonFeatureModule(t, "errorcontext", tc.variant)
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
		out, stderr, err = goCommand(t, tc.tool, tc.baseline, dir, "test", "-vet=off", "-cover", "-count=1", ".")
		if err != nil {
			t.Fatalf("%s baseline=%v coverage: %v\n%s%s", tc.variant, tc.baseline, err, out, stderr)
		}
	}
}
