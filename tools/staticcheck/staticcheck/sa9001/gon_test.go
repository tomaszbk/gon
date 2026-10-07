package sa9001

import (
	"golang.org/x/tools/go/analysis/analysistest"
	"path/filepath"
	"testing"
)

func TestGon(t *testing.T) {
	dir, err := filepath.Abs("gontestdata")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, dir, Analyzer, "gonfixture")
}
