package cmd_test

import (
	"strings"
	"testing"
)

func TestGonNilAwayOptIn(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, `-- go.mod --
module example.com/nilanalysis

go 1.27
-- p.go --
package p
func missing() *int { return nil }
func dereference() int { return *missing() }
`)
	var disabled gonCheck
	gonJSON(t, tree, nil, &disabled, "check", "./...").checkCode(0)
	for _, diagnostic := range disabled.Diagnostics {
		if diagnostic.Source == "nilaway" {
			t.Fatalf("NilAway enabled by default: %+v", disabled)
		}
	}
	if !strings.Contains(strings.Join(disabled.NotVerified, "\n"), "NilAway") {
		t.Fatalf("missing opt-in verification record: %+v", disabled)
	}
	var enabled gonCheck
	gonJSON(t, tree, nil, &enabled, "check", "./...", "--nilaway").checkCode(0)
	found := false
	for _, diagnostic := range enabled.Diagnostics {
		if diagnostic.Source == "nilaway" {
			found = true
			if diagnostic.Severity != "warning" {
				t.Errorf("NilAway severity: %+v", diagnostic)
			}
			if strings.Contains(diagnostic.Message, "\x1b[") || strings.HasPrefix(diagnostic.Message, "error:") {
				t.Errorf("NilAway message includes terminal styling or wrong severity: %q", diagnostic.Message)
			}
		}
	}
	if !found {
		t.Fatalf("NilAway did not report interprocedural nil: %+v", enabled)
	}
	if strings.Contains(strings.Join(enabled.NotVerified, "\n"), "NilAway") {
		t.Fatalf("enabled NilAway marked unverified: %+v", enabled)
	}
}

func TestGonNilAwayDependencyDiagnostic(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, `-- go.mod --
module example.com/nilanalysis

go 1.27
-- library/library.go --
package library
func Dereference(p *int) int { return *p }
-- app/app.go --
package app
import "example.com/nilanalysis/library"
func Bad() int { return library.Dereference(nil) }
`)
	var result gonCheck
	gonJSON(t, tree, nil, &result, "check", "./app", "--nilaway").checkCode(0)
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Source == "nilaway" && strings.HasSuffix(diagnostic.Location.Path, "/library/library.go") {
			found = true
			if diagnostic.Location.Line != 2 || diagnostic.Location.Column != 40 {
				t.Errorf("dependency dereference position: %+v", diagnostic.Location)
			}
		}
	}
	if !found {
		t.Fatalf("NilAway dropped the dependency's dereference location: %+v", result)
	}
}

func TestGonNilAwayExportedInterfacePayload(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, `-- go.mod --
module example.com/nilanalysis

go 1.27
-- library/library.go --
package library
var marker int?
func Check(x any) int {
 switch p := x.(type) { case *int: /*界*/ return *p }
 return 0
}
func localSafe() int { return Check(new(int)) }
-- safe/safe.go --
package safe
import "example.com/nilanalysis/library"
func Good() int { return library.Check(new(int)) }
func NilInterface() int { return library.Check(nil) }
-- bad/bad.go --
package bad
import "example.com/nilanalysis/library"
func Bad() int { return library.Check((*int)(nil)) }
func FunctionValue() int { check := library.Check; return check((*int)(nil)) }
`)
	var safe gonCheck
	gonJSON(t, tree, nil, &safe, "check", "./safe", "--nilaway").checkCode(0)
	for _, diagnostic := range safe.Diagnostics {
		if diagnostic.Source == "nilaway" {
			t.Fatalf("safe interface payload warned: %+v", safe)
		}
	}
	var bad gonCheck
	gonJSON(t, tree, nil, &bad, "check", "./bad", "--nilaway").checkCode(0)
	found := false
	for _, diagnostic := range bad.Diagnostics {
		if diagnostic.Source == "nilaway" && strings.Contains(diagnostic.Message, "dereferenced") {
			found = true
			if !strings.HasSuffix(diagnostic.Location.Path, "/library/library.go") || diagnostic.Location.Line != 4 || diagnostic.Location.Column != 52 {
				t.Errorf("exported payload dereference position: %+v", diagnostic.Location)
			}
		}
	}
	if !found {
		t.Fatalf("typed-nil interface payload escaped interpackage analysis: %+v", bad)
	}
}

func TestGonNilAwayCustomSeq(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, `-- go.mod --
module gon

go 1.27
-- seq/seq.go --
package seq
func First(xs []*int) (*int)? { return (*int)(nil) }
-- app/app.go --
package app
import "gon/seq"
func Bad() {
 if value := seq.First([]*int{new(int)}); value is p? { _ = *p }
}
`)
	var result gonCheck
	gonJSON(t, tree, nil, &result, "check", "./app", "--nilaway").checkCode(0)
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Source == "nilaway" && strings.Contains(diagnostic.Message, "dereferenced") {
			found = true
		}
	}
	if !found {
		t.Fatalf("custom gon/seq was mistaken for the standard package: %+v", result)
	}
}
