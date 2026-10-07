package misc

import (
	"strings"
	"testing"

	. "golang.org/x/tools/gopls/internal/test/integration"
)

func TestGonPatternBindingEditor(t *testing.T) {
	const source = `
-- go.mod --
module example.com/patterneditor

go 1.27
-- p.go --
package p

func choose(value int?) int {
	if value is number? && number > 0 {
		return number
	} else {
		return 0
	}
}
`
	WithOptions(Modes(Default)).Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("p.go")
		for _, at := range []string{`&& num()ber`, `return num()ber`} {
			found := false
			for _, item := range env.Completion(env.RegexpSearch("p.go", at)).Items {
				found = found || item.Label == "number"
			}
			if !found {
				t.Fatalf("pattern binding unavailable at %s", at)
			}
		}
		for _, item := range env.Completion(env.RegexpSearch("p.go", `return ()0`)).Items {
			if item.Label == "number" {
				t.Fatal("pattern binding leaked into else")
			}
		}
		hover, _ := env.Hover(env.RegexpSearch("p.go", `return (number)`))
		if hover == nil || !strings.Contains(hover.Value, "number int") {
			t.Fatalf("pattern binding hover: %+v", hover)
		}
		env.Rename(env.RegexpSearch("p.go", `is (number)`), "count")
		got := env.BufferText("p.go")
		if strings.Contains(got, "number") || strings.Count(got, "count") != 3 {
			t.Fatalf("pattern binding rename: %s", got)
		}
	})
}

func TestGonAlternativeBindingRename(t *testing.T) {
	const source = `
-- go.mod --
module example.com/matcheditor

go 1.27
-- p.go --
package p

type Choice enum { default Empty; First(int); Second(int) }
func choose(value Choice) int { return switch value {
case Choice.First(number), Choice.Second(number) => number
case Choice.Empty => 0
} }
`
	WithOptions(Modes(Default)).Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("p.go")
		env.Rename(env.RegexpSearch("p.go", `Second\((number)`), "count")
		got := env.BufferText("p.go")
		if strings.Contains(got, "number") || strings.Count(got, "count") != 3 {
			t.Fatalf("alternative binding rename: %s", got)
		}
	})
}

func TestGonInterpolationEditor(t *testing.T) {
	const source = "\n-- go.mod --\nmodule example.com/stringeditor\n\ngo 1.27\n-- p.go --\npackage p\nimport text \"fmt\"\n\nfunc describe(value int) string {\n\treturn $`first\nsecond ${value} / ${value /* comment: keep */:%d}`\n}\n"
	WithOptions(Modes(Default), Settings{"semanticTokens": true}).Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("p.go")
		hover, _ := env.Hover(env.RegexpSearch("p.go", `\$\{(value)\}`))
		if hover == nil || !strings.Contains(hover.Value, "value int") {
			t.Fatalf("interpolation hover: %+v", hover)
		}
		values, opens, rawLines, formats, comments := 0, 0, 0, 0, 0
		for _, token := range env.SemanticTokensFull("p.go") {
			if token.Token == "value" {
				values++
				if token.TokenType != "parameter" {
					t.Errorf("interpolated parameter token: %+v", token)
				}
			}
			if token.Token == "${" && token.TokenType == "operator" {
				opens++
			}
			if token.Token == ":%d" && token.TokenType == "string" {
				formats++
			}
			if token.Token == "/* comment: keep */" && token.TokenType == "comment" {
				comments++
			}
			if token.TokenType == "string" && (token.Token == "first" || token.Token == "second ") {
				rawLines++
			}
		}
		if values != 3 || opens != 2 || rawLines != 2 || formats != 1 || comments != 1 {
			t.Fatalf("semantic token counts: parameters=%d openings=%d raw lines=%d formats=%d comments=%d", values, opens, rawLines, formats, comments)
		}
		env.Rename(env.RegexpSearch("p.go", `\$\{(value)\}`), "count")
		got := env.BufferText("p.go")
		if strings.Contains(got, "value") || strings.Count(got, "count") != 3 {
			t.Fatalf("interpolated parameter rename: %s", got)
		}
	})
}
