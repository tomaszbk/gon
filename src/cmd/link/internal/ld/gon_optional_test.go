package ld

import (
	"os/exec"
	"testing"
)

func TestGonOptionalShapeDWARF(t *testing.T) {
	mustHaveDWARF(t)
	f := gobuild(t, t.TempDir(), `package main
import "gon/seq"
func main() {
 if (seq.Lookup(map[string]int{"x": 1}, "x") ?? 0) != 1 { panic("plain payload") }
 outer := seq.Lookup(map[string]int?{"x": nil, "y": 2}, "x")
 if outer == nil || (outer ?? nil) != nil { panic("present absent payload") }
 if ((seq.Lookup(map[string]int?{"y": 2}, "y") ?? nil) ?? 0) != 2 { panic("nested payload") }
}`, DefaultOpt)
	defer f.Close()
	if _, err := f.DWARF(); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(f.path).CombinedOutput(); err != nil {
		t.Fatalf("execution: %v\n%s", err, out)
	}
}
