package errors

import _ "unsafe" // for linkname

// ErrNilResult is the error returned by postfix ! when it propagates a failed
// Result whose Err payload is nil to a function whose last result is error.
// A failed Result is always a failure, so a nil payload is never returned as
// a nil error. Test for it with [Is]:
//
//	if errors.Is(err, errors.ErrNilResult) { ... }
var ErrNilResult = nilResult()

// nilResult is provided by package runtime, which cannot import this package.
//
//go:linkname nilResult
func nilResult() error
