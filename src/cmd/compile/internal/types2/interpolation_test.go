package types2_test

import (
	"cmd/compile/internal/syntax"
	. "cmd/compile/internal/types2"
	"strings"
	"testing"
)

func TestInterpolationTypes(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{`import "fmt";func f(x int)string{return $"${x:%04d}"}`, ""},
		{`import alias "fmt";func f()string{alias:=1;return $"${alias}"}`, ""},
		{`import _ "fmt";var _ = $"empty"`, ""},
		{`func f()string{return $"${1}"}`, "requires an explicit import"},
		{`import "fmt";const x = $"${1}"`, "not constant"},
		{`import "fmt";var _ = $"${1:%%}"`, "one fmt verb"},
		{`import "fmt";var _ = $"${1:%*d}"`, "one fmt verb"},
		{`import "fmt";var _ = $"${1:%[2]d}"`, "one fmt verb"},
		{`import "fmt";var _ = $"bad\z"`, "invalid interpolated string escape"},
	} {
		errs := checkFile(t, "source.go", "package p;"+tc.source)
		joined := strings.Join(errs, "\n")
		if tc.want == "" && len(errs) != 0 || tc.want != "" && !strings.Contains(joined, tc.want) {
			t.Fatalf("%s: got %s want %s", tc.source, joined, tc.want)
		}
	}
}

// Line directives change diagnostics, never the physical file that imports fmt.
func TestInterpolationImportFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []string
		want  string
	}{
		{"same file", []string{"package p\nimport \"fmt\"\n//line generated.go:20\nfunc f() string { return $\"${1}\" }\n"}, ""},
		{"different file", []string{"package p\nimport \"fmt\"\nvar _ = fmt.Sprintf\n", "package p\n//line fmt.go:50\nvar _ = $\"${1}\"\n"}, "requires an explicit import"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var files []*syntax.File
			for i, source := range tc.files {
				name := "fmt.go"
				if i > 0 {
					name = "missing.go"
				}
				file, err := syntax.Parse(syntax.NewFileBase(name), strings.NewReader(source), nil, nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				files = append(files, file)
			}
			_, err := (&Config{Importer: defaultImporter()}).Check("p", files, nil)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}
