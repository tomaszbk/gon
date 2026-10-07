// Copyright 2026 The Gon Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package diagnostic

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/tokenhelper"
	"golang.org/x/tools/go/analysis"
)

func TestGonDiagnosticOrderAcrossSourcePathForms(t *testing.T) {
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "diagnostic_order.go")
	fset := token.NewFileSet()
	source, err := parser.ParseFile(fset, filename, "package p\nvar early = 1\nvar middle = 2\nvar late = 3\n", parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	pass := analysishelper.NewEnhancedPass(&analysis.Pass{
		Fset:  fset,
		Files: []*ast.File{source},
		ResultOf: map[*analysis.Analyzer]any{
			config.Analyzer: &config.Config{},
			NoLintAnalyzer:  &analysishelper.Result[[]Range]{},
		},
	})
	var positions []token.Position
	for _, declaration := range source.Decls {
		positions = append(positions, fset.Position(declaration.Pos()))
	}

	for _, grouping := range []bool{false, true} {
		engine := NewEngine(pass)
		// Direct conflicts use a relative filename; inferred conflicts carry
		// an absolute filename from facts. Their offsets must still determine
		// source order, regardless of the original conflict insertion order.
		for _, index := range []int{2, 0, 1} {
			position := positions[index]
			if index != 1 {
				position.Filename = tokenhelper.RelToCwd(position.Filename)
			}
			engine.conflicts = append(engine.conflicts, conflict{
				position: position,
				flow: nilFlow{nonnilPath: []node{{
					producerRepr: []string{"early", "middle", "late"}[index],
				}}},
			})
		}
		diagnostics := engine.Diagnostics(grouping)
		if len(diagnostics) != len(positions) {
			t.Fatalf("grouping %v: got %d diagnostics, want %d", grouping, len(diagnostics), len(positions))
		}
		for index, diagnostic := range diagnostics {
			if actual := fset.Position(diagnostic.Pos).Offset; actual != positions[index].Offset {
				t.Errorf("grouping %v: diagnostic %d offset = %d, want %d", grouping, index, actual, positions[index].Offset)
			}
		}
	}
}

func TestGonDiagnosticSourcePositions(t *testing.T) {
	const upstreamSource = "package upstream\n\nfunc Value() *int {\n\treturn nil\n}\n"
	const ownedSource = "package owned\n\nvar Present = 3\n"
	directory := t.TempDir()
	upstreamName := filepath.Join(directory, "upstream.go")
	if err := os.WriteFile(upstreamName, []byte(upstreamSource), 0600); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	// Export readers preserve columns using sparse offsets. The gaps between
	// these line starts are unrelated to the real source's byte offsets.
	exportFile := fset.AddFile(upstreamName, -1, 4096)
	if !exportFile.SetLines([]int{0, 256, 512, 768, 1024}) {
		t.Fatal("could not create sparse export positions")
	}
	ownedName := filepath.Join(directory, "owned.go")
	owned, err := parser.ParseFile(fset, ownedName, ownedSource, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var reads []string
	pass := analysishelper.NewEnhancedPass(&analysis.Pass{
		Fset:  fset,
		Files: []*ast.File{owned},
		ReadFile: func(filename string) ([]byte, error) {
			reads = append(reads, filename)
			return os.ReadFile(filename)
		},
	})
	engine := NewEngine(pass)

	upstreamPosition := exportFile.Position(exportFile.Pos(768 + 8))
	upstreamPosition.Filename = tokenhelper.RelToCwd(upstreamName)
	actualPos := engine.toPos(upstreamPosition)
	actual := fset.Position(actualPos)
	expectedOffset := strings.Index(upstreamSource, "nil")
	if actual.Filename != upstreamName || actual.Line != 4 || actual.Column != 9 || actual.Offset != expectedOffset {
		t.Fatalf("diagnostic uses export offsets instead of source: got %v (offset %d), want %s:4:9 (offset %d)",
			actual, actual.Offset, upstreamName, expectedOffset)
	}
	if fset.File(actualPos) == exportFile {
		t.Fatal("sparse export file was treated as an owned source file")
	}
	if repeated := engine.toPos(upstreamPosition); repeated != actualPos {
		t.Fatalf("repeated diagnostic changed source position: %v then %v", actualPos, repeated)
	}

	// Actual package AST files already have correct offsets and must keep
	// their original token.Pos without a ReadFile attempt or replacement.
	present := owned.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0].Pos()
	ownedPosition := fset.Position(present)
	ownedPosition.Filename = tokenhelper.RelToCwd(ownedName)
	if actual := engine.toPos(ownedPosition); actual != present {
		t.Fatalf("owned AST position changed: got %v, want %v", actual, present)
	}
	if len(reads) != 1 || reads[0] != upstreamName {
		t.Fatalf("ReadFile calls = %v; want only %s once", reads, upstreamName)
	}
}
