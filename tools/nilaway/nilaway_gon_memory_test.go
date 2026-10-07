package nilaway

import (
	"golang.org/x/tools/go/analysis/analysistest"
	"testing"
)

func TestGonMemory(t *testing.T) {
	t.Parallel()
	analysistest.Run(t, analysistest.TestData(), Analyzer, "gon.test/gonmemory")
}
