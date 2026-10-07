// Copyright (c) 2026 Gon contributors.
// Licensed under the Apache License, Version 2.0.

package inference

import (
	"bytes"
	"encoding/gob"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

// A dependency's analysis facts may be reused by clients in different working
// directories. Its diagnostic must still identify the original source file.
func TestGonFactSourcePathStable(t *testing.T) {
	root := t.TempDir()
	clientA := filepath.Join(root, "client-a")
	clientB := filepath.Join(root, "client-b")
	for _, dir := range []string{clientA, clientB} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(root, "dependency.go")
	content := []byte("package dependency\nvar Value *int\n")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file := fset.AddFile(source, -1, len(content))
	file.SetLinesForContent(content)
	pos := file.Pos(bytes.Index(content, []byte("Value")))
	// The identity uses real source locations even in generated files that
	// carry an unrelated //line destination.
	file.AddLineColumnInfo(0, "generated-alias.go", 100, 1)

	t.Chdir(clientA)
	p := primitivizer{pass: analysishelper.NewEnhancedPass(&analysis.Pass{Fset: fset})}
	exported := primitiveFullTrigger{Position: p.toPosition(pos)}
	var cache bytes.Buffer
	if err := gob.NewEncoder(&cache).Encode(exported); err != nil {
		t.Fatal(err)
	}

	t.Chdir(clientB)
	var imported primitiveFullTrigger
	if err := gob.NewDecoder(&cache).Decode(&imported); err != nil {
		t.Fatal(err)
	}
	if imported.Position.Filename != source {
		t.Fatalf("cached source path = %q, want original %q", imported.Position.Filename, source)
	}
	if imported.Position.Line != 2 || imported.Position.Column != 5 {
		t.Fatalf("cached source position = %v, want line 2, column 5", imported.Position)
	}
	read, err := os.ReadFile(imported.Position.Filename)
	if err != nil {
		t.Fatalf("client B cannot read the cached dependency source: %v", err)
	}
	if !bytes.Equal(read, content) {
		t.Fatalf("cached source path resolved to a different file: %q", read)
	}
}
