package nilaway

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestGonGoPrecision(t *testing.T) {
	t.Parallel()
	analysistest.Run(t, analysistest.TestData(), Analyzer, "gon.test/gonprecision", "gon.test/gonprecisionlegacy", "gon.test/gonprecisionunsafe", "gon.test/gonprecisionunsafelegacy")
}
