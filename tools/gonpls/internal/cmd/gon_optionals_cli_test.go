package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGonOptionalMigrationPlan(t *testing.T) {
	tree := writeTree(t, `
-- go.mod --
module example.com/optional

go 1.27
-- main.go --
package main
import "fmt"
type Number = Option[int]
var constructor = Option[int].Some
func main() {
 var n Number = .Some(7)
 var p Option[*int] = .Some(nil)
 var nested Option[Option[int]] = .Some(.None)
 fmt.Println(n ?? 0, switch p { case Option[*int].Some(_) => true; case Option[*int].None => false }, match(nested), constructor(3) ?? 0)
}
-- patterns.go --
package main
func match(n Option[Option[int]]) int {
 return switch n {
 case Option[Option[int]].None => 0
 case Option[Option[int]].Some(Option[int].None) => 1
 case Option[Option[int]].Some(Option[int].Some(v)) => v
 }
}
-- custom/custom.go --
package custom
type Option[T any] struct { Some T; None bool }
var _ = Option[int]{Some: 3}
`)
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(tree, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	before, patterns, custom := read("main.go"), read("patterns.go"), read("custom/custom.go")
	var plan gonPlanResult
	res := gonJSON(t, tree, nil, &plan, "refactor", "optionals", "./...")
	res.checkCode(0)
	if plan.Status != "planned" || plan.Summary.Files != 2 || read("main.go") != before || read("patterns.go") != patterns {
		t.Fatalf("default preview modified source or wrong scope: %+v", plan)
	}
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, []byte(res.stdout), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "patterns.go"), []byte(patterns+"\n// concurrent edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stale gonPlanResult
	gonJSON(t, tree, nil, &stale, "refactor", "apply", planFile).checkCode(1)
	if stale.Error == nil || stale.Error.Kind != "stale" || read("main.go") != before || read("patterns.go") != patterns+"\n// concurrent edit\n" {
		t.Fatalf("stale optional plan did not preserve all files: %+v", stale)
	}
	res = gonJSON(t, tree, nil, &plan, "refactor", "optionals", "./...")
	res.checkCode(0)
	if err := os.WriteFile(planFile, []byte(res.stdout), 0600); err != nil {
		t.Fatal(err)
	}
	gon(t, tree, nil, "refactor", "apply", planFile).checkCode(0)
	if read("custom/custom.go") != custom || strings.Contains(read("main.go"), "Option[") || strings.Contains(read("patterns.go"), ".Some") || !strings.Contains(read("patterns.go"), "(v?)?") {
		t.Fatal("migration changed user names or retained retired constructors")
	}
	gon(t, tree, nil, "refactor", "apply", planFile).checkCode(1)
	run := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "run", ".")
	run.Dir = tree
	run.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	output, err := run.CombinedOutput()
	if err != nil || string(output) != "7 true 1 3\n" {
		t.Fatalf("native migration execution: %v\n%s", err, output)
	}
	gonJSON(t, tree, nil, &plan, "refactor", "optionals", "./...").checkCode(0)
	if plan.Summary.Files != 0 {
		t.Fatalf("migration is not idempotent: %+v", plan)
	}
}
