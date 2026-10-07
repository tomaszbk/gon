package gonmemory

import (
	"gon.test/gonapi"
	"testing"
)

func fromTest(t *testing.T) *int    { return gonapi.Outcome(false)! }
func TestFatalPointer(t *testing.T) { value := fromTest(t); _ = *value }
