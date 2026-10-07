package printer_test

import (
	"bytes"
	"go/format"
	"testing"
)

func TestPatternTestControlParens(t *testing.T) {
	const source = `package p
type Rec enum { default Empty; Value{N int} }
func f(s Rec) {
 for i:=0; (s is Rec.Value{N: 1}); i++ {}
 if (s is Rec.Value{N: 1}) {}
}
`
	formatted, err := format.Source([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(formatted, []byte("(s is Rec.Value{N: 1})")) {
		t.Fatalf("required parentheses removed:\n%s", formatted)
	}
	again, err := format.Source(formatted)
	if err != nil || !bytes.Equal(formatted, again) {
		t.Fatalf("round trip: %v\n%s", err, again)
	}
}
