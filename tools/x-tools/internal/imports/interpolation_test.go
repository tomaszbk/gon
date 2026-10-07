package imports

import (
	"golang.org/x/tools/internal/packagestest"
	"testing"
)

func TestGonInterpolationImports(t *testing.T) {
	for _, pair := range []struct{ name, input, want string }{
		{"add", `package main
var _ = $"hello ${1}"
`, `package main

import "fmt"

var _ = $"hello ${1}"
`},
		{"alias", `package main
import text "fmt"
var _ = $"hello ${1}"
`, `package main

import text "fmt"

var _ = $"hello ${1}"
`},
		{"shadow", `package main
import "fmt"
func f() string { fmt:=1; return $"${fmt}" }
`, `package main

import "fmt"

func f() string { fmt := 1; return $"${fmt}" }
`},
	} {
		t.Run(pair.name, func(t *testing.T) {
			testConfig{module: packagestest.Module{Name: "example.com/interpolation", Files: fm{"main.go": "package main\n"}}}.processTest(t, "example.com/interpolation", "main.go", []byte(pair.input), nil, pair.want)
		})
	}
}
