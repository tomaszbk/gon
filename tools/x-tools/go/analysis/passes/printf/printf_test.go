// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printf_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/internal/testenv"
	"golang.org/x/tools/internal/testfiles"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	printf.Analyzer.Flags.Set("funcs", "Warn,Warnf")

	analysistest.Run(t, testdata, printf.Analyzer,
		"a", "b", "nofmt", "nonconst", "typeparams", "issue62595", "issue68744", "issue70572", "issue72850", "issue76616", "w")
}

func TestNonConstantFmtString_Go123(t *testing.T) {
	testenv.NeedsGo1Point(t, 23)

	dir := testfiles.ExtractTxtarFileToTmp(t, filepath.Join(analysistest.TestData(), "nonconst_go123.txtar"))
	analysistest.RunWithSuggestedFixes(t, dir, printf.Analyzer, "example.com/nonconst")
}

func TestNonConstantFmtString_Go124(t *testing.T) {
	testenv.NeedsGo1Point(t, 24)

	dir := testfiles.ExtractTxtarFileToTmp(t, filepath.Join(analysistest.TestData(), "nonconst_go124.txtar"))
	analysistest.RunWithSuggestedFixes(t, dir, printf.Analyzer, "example.com/nonconst")
}

func TestGon(t *testing.T) {
	results := analysistest.Run(t, analysistest.TestData(), printf.Analyzer, "gonfixture")
	for _, result := range results {
		for _, diagnostic := range result.Diagnostics {
			if !strings.HasPrefix(diagnostic.Message, "fmt.Sprintf format") {
				continue
			}
			file := result.Pass.Fset.File(diagnostic.Pos)
			source, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			got := string(source[file.Offset(diagnostic.Pos):file.Offset(diagnostic.End)])
			if got != "%d" && got != "%s" {
				t.Errorf("wrong interpolation diagnostic range %q at %s", got, result.Pass.Fset.Position(diagnostic.Pos))
			}
		}
	}
}
