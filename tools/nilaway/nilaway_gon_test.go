package nilaway

import (
	"golang.org/x/tools/go/analysis/analysistest"
	"testing"
)

// Gon and the ordinary Go counterpart must retain the same nilness outcomes;
// only genuine dereferences are expected, with no package silently skipped.
func TestGonNilAway(t *testing.T) {
	t.Parallel()
	analysistest.Run(t, analysistest.TestData(), Analyzer,
		"gon.test/gonsafe", "gon.test/gonlegacy", "gon.test/gonunsafe", "gon.test/gonunsafelegacy")
}
