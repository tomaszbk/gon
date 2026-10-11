package misc

import (
	. "golang.org/x/tools/gopls/internal/test/integration"
	"strings"
	"testing"
)

func TestGonModuleVersionEditor(t *testing.T) {
	const source = `
-- go.mod --
module example.com/gonversion

go 2.27
-- p.go --
package p
func Double(value int?) int {
    if value is number? { return number * 2 }
    return 0
}
`
	WithOptions(Modes(Default)).Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("p.go")
		env.AfterChange(NoDiagnostics(ForFile("p.go")))
		hover, _ := env.Hover(env.RegexpSearch("p.go", `return (number)`))
		if hover == nil || !strings.Contains(hover.Value, "number int") {
			t.Fatalf("hover: %+v", hover)
		}
		env.Rename(env.RegexpSearch("p.go", `is (number)`), "count")
		if text := env.BufferText("p.go"); strings.Contains(text, "number") || strings.Count(text, "count") != 2 {
			t.Fatal(text)
		}
	})
}
