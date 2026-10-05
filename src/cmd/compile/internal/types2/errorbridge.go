package types2

import "strings"

// propagationTarget classifies the nearest enclosing function of a postfix !
// propagation: how a failure leaves it.
type propagationTarget int

const (
	noPropagation     propagationTarget = iota
	propagateToError                    // the last result is exactly error
	propagateToResult                   // the only result is a canonical Result
	propagateToTest                     // a test function reports the failure with Fatal
)

// propagationTarget reports the target of postfix ! in a function with
// signature sig. A function that already returns error last or exactly one
// Result never uses the test-function rule. The test-function rule needs a
// _test.go file at the position at, and a named first parameter of type
// *testing.T, *testing.B, *testing.F or testing.TB.
func (check *Checker) propagationTarget(sig *Signature, at poser) propagationTarget {
	if sig == nil {
		return noPropagation
	}
	n := sig.results.Len()
	if n > 0 && Identical(sig.results.At(n-1).typ, universeError) {
		return propagateToError
	}
	if n == 1 && IsCanonicalResult(sig.results.At(0).typ) {
		return propagateToResult
	}
	if strings.HasSuffix(at.Pos().FileBase().Filename(), "_test.go") && testingParam(sig) != nil {
		return propagateToTest
	}
	return noPropagation
}

// testingParam returns the first parameter of sig when it is named, not blank
// and has type *testing.T, *testing.B, *testing.F or testing.TB from the
// standard testing package (aliases included). A method receiver is not a
// parameter.
func testingParam(sig *Signature) *Var {
	if sig.params.Len() == 0 {
		return nil
	}
	p := sig.params.At(0)
	if p.name == "" || p.name == "_" {
		return nil
	}
	t := Unalias(p.typ)
	pointer := false
	if ptr, ok := t.(*Pointer); ok {
		t, pointer = Unalias(ptr.base), true
	}
	named, _ := t.(*Named)
	if named == nil {
		return nil
	}
	obj := named.Origin().obj
	if obj.pkg == nil || obj.pkg.path != "testing" {
		return nil
	}
	if pointer && (obj.name == "T" || obj.name == "B" || obj.name == "F") || !pointer && obj.name == "TB" {
		return p
	}
	return nil
}

// enclosingResultError returns the error type of the Result returned by sig.
func enclosingResultError(sig *Signature) Type {
	return canonicalPayload(sig.results.At(0).typ, "Err")
}
