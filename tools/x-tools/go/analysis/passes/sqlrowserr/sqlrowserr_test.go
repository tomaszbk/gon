// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sqlrowserr_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/sqlrowserr"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.RunWithSuggestedFixes(t, testdata, sqlrowserr.Analyzer, "a")
}

// TestGon checks that Gon "!" and "or" handlers on a query are analyzed
// like the Go assignment form.
func TestGon(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), sqlrowserr.Analyzer, "gon")
}
