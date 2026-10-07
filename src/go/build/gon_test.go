package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGonPackagePrecedence(t *testing.T) {
	t.Setenv("GO111MODULE", "off")
	root := t.TempDir()
	ctxt := Default
	ctxt.GOROOT, ctxt.GOPATH = filepath.Join(root, "goroot"), filepath.Join(root, "gopath")
	standard := filepath.Join(ctxt.GOROOT, "src", "gon", "seq")
	local := filepath.Join(ctxt.GOPATH, "src", "gon", "seq")
	for _, dir := range []string{standard, local} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(standard, "seq.go"), []byte("package seq\n"), 0600); err != nil {
		t.Fatal(err)
	}
	check := func(wantDir string, wantGoroot bool) {
		t.Helper()
		pkg, err := ctxt.Import("gon/seq", "", FindOnly)
		if err != nil || pkg.Dir != wantDir || pkg.Goroot != wantGoroot {
			t.Fatalf("Import = %+v, %v; want directory %s, Goroot %v", pkg, err, wantDir, wantGoroot)
		}
	}
	// A directory without Go files does not claim the new package path.
	check(standard, true)
	if err := os.WriteFile(filepath.Join(local, "seq.go"), []byte("package seq\n"), 0600); err != nil {
		t.Fatal(err)
	}
	check(local, false)
}
