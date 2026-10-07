package types2

// propagationTarget classifies how a postfix ! failure leaves its enclosing function.
type propagationTarget int

const (
	noPropagation    propagationTarget = iota
	propagateToError                   // the last result is exactly error
)

// propagationTarget reports whether the nearest enclosing function returns error last.
func (check *Checker) propagationTarget(sig *Signature) propagationTarget {
	if sig != nil {
		n := sig.results.Len()
		if n > 0 && Identical(sig.results.At(n-1).typ, universeError) {
			return propagateToError
		}
	}
	return noPropagation
}
