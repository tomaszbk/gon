// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httpresponse_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, httpresponse.Analyzer, "a", "typeparams")
}

// TestGon checks that Gon error propagation ("!") and terminating "or"
// handlers count as the error check before a deferred Body.Close, while
// the Go diagnostics remain reported.
func TestGon(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), httpresponse.Analyzer, "gonfixture")
}
