package types

import "strings"

// propagationTarget classifies the nearest enclosing function of a postfix !
// propagation: how a failure leaves it.
type propagationTarget int

const (
	noPropagation    propagationTarget = iota
	propagateToError                   // the last result is exactly error
	propagateToTest                    // a test function reports the failure with Fatal
)

// propagationTarget reports the target of postfix ! in a function with
// signature sig. A function that already returns error last never uses the
// test-function rule. The test-function rule needs a
// _test.go file at the position at, and a named first parameter of type
// *testing.T, *testing.B, *testing.F or testing.TB.
func (check *Checker) propagationTarget(sig *Signature, at positioner) propagationTarget {
	if sig == nil {
		return noPropagation
	}
	n := sig.results.Len()
	if n > 0 && Identical(sig.results.At(n-1).typ, universeError) {
		return propagateToError
	}
	if check.fset != nil && strings.HasSuffix(check.fset.PositionFor(at.Pos(), false).Filename, "_test.go") && TestFatalParam(sig) != nil {
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

// TestFatalParam returns the parameter through which postfix ! in a function
// with signature sig reports a failure, or nil. That is the first parameter,
// when it is named, not blank and has type *testing.T, *testing.B, *testing.F
// or testing.TB from the standard testing package, and the function does not
// return error last. The rule applies only in _test.go files, which the caller
// checks.
func TestFatalParam(sig *Signature) *Var {
	if sig == nil {
		return nil
	}
	if n := sig.results.Len(); n > 0 && Identical(sig.results.At(n-1).typ, universeError) {
		return nil
	}
	return testingParam(sig)
}
