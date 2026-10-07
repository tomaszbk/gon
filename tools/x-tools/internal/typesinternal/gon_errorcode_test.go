package typesinternal_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/internal/typesinternal"
)

// TestGonErrorCodes checks, unlike the skipped upstream test, that every
// type-checker code of the selected toolchain has the same value here, so
// that gonpls reports stable code names such as InvalidErrorHandling.
func TestGonErrorCodes(t *testing.T) {
	std, err := loadCodes(filepath.Join(runtime.GOROOT(), "src", "internal", "types", "errors", "codes.go"))
	if err != nil {
		t.Fatal(err)
	}
	local, err := loadCodes("errorcode.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := std["InvalidErrorHandling"]; !ok {
		t.Fatal("the selected GOROOT is not a Gon toolchain")
	}
	for name, value := range std {
		if got, ok := local[name]; !ok || got != value {
			t.Errorf("%s: got %d (present %v), toolchain has %d", name, got, ok, value)
		}
	}
	for _, name := range []string{"InvalidErrorHandling", "InvalidLambda", "InvalidNilSafety", "InvalidMatch", "InvalidInterpolation"} {
		if got := typesinternal.ErrorCode(std[name]).String(); got != name {
			t.Errorf("%s.String() = %q", name, got)
		}
	}
}
