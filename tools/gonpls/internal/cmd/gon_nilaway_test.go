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
		}
	}
	if !found {
		t.Fatalf("NilAway did not report interprocedural nil: %+v", enabled)
	}
	if strings.Contains(strings.Join(enabled.NotVerified, "\n"), "NilAway") {
		t.Fatalf("enabled NilAway marked unverified: %+v", enabled)
	}
}
