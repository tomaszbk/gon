package gonmemory

import (
	"gon.test/gonapi"
	"testing"
)

func fromTest(t *testing.T) (*int, error) { return gonapi.Outcome(false)!, nil }
func TestReturnPointer(t *testing.T) {
	value, err := fromTest(t)
	if err != nil {
		t.Fatal(err)
		return
	}
	_ = *value
}
