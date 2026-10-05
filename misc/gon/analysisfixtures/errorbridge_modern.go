package main

// Postfix ! across Go error tuples and Result.

type failure struct{ text string }

func (f failure) Error() string { return f.text }

type coded struct{ code int }

func (c *coded) Error() string { return "coded" }

type nilError interface{ Error() string }

type IntResult = Result[int, error]
type AnyResult = Result[int, any]
type CodedResult = Result[int, *coded]
type NilErrorResult = Result[int, nilError]

var trace string

func mark(s string) { trace += s }
func check(ok bool) {
	if !ok {
		panic("errorbridge mismatch " + trace)
	}
}

func source(label string, n int, bad bool) (int, error) {
	mark(label)
	if bad {
		return n, failure{"bad"}
	}
	return n, nil
}
func pair(bad bool) (int, string, error) {
	mark("p")
	if bad {
		return 40, "partial", failure{"pair"}
	}
	return 41, "ok", nil
}
func only(bad bool) error {
	mark("o")
	if bad {
		return failure{"only"}
	}
	return nil
}

func provide(kind int) IntResult {
	mark("r")
	switch kind {
	case 0:
		return .Ok(7)
	case 1:
		return .Err(failure{"provided"})
	}
	return .Err(nil)
}

// An error tuple fails as Err inside a function returning one Result.
func tupleOne(bad bool) IntResult {
	n := source("a", 5, bad)!
	return .Ok(n * 2)
}
func tupleMany(bad bool) IntResult {
	n, s := pair(bad)!
	return .Ok(n + len(s))
}
func tupleOnly(bad bool) IntResult {
	only(bad)!
	mark("after")
	return .Ok(1)
}
func tupleAny(bad bool) AnyResult {
	n := source("a", 5, bad)!
	return .Ok(n)
}
func tupleNamed(bad bool) (r IntResult) {
	r = .Ok(100)
	defer func() { mark("D") }()
	n := source("a", 1, bad)!
	return .Ok(n)
}
func tupleOrder(failAt int) IntResult {
	a := 1
	mark("x")
	return .Ok(a + source("b", 2, failAt == 2)! + source("c", 3, failAt == 3)!)
}

// A failed Result fails as its error inside a function returning error last.
func resultToError(kind int) (int, error) {
	n := provide(kind)!
	return n + 1, nil
}
func resultToErrorMany(kind int) (string, int, error) {
	n := provide(kind)!
	return "ok", n, nil
}
func resultToErrorOnly(kind int) error {
	provide(kind)!
	return nil
}
func resultToErrorNamed(kind int) (n int, err error) {
	n = 99
	defer func() { mark("D") }()
	v := provide(kind)!
	return v, nil
}
func resultToErrorCoded(nilPayload bool) (int, error) {
	var r CodedResult = .Err(&coded{3})
	if nilPayload {
		r = .Err(nil)
	}
	n := r!
	return n, nil
}
func resultToErrorInterface(nilPayload bool) (int, error) {
	var r NilErrorResult = .Err(failure{"iface"})
	if nilPayload {
		r = .Err(nil)
	}
	n := r!
	return n, nil
}

func main() {
	check(tupleOne(false) == IntResult.Ok(10) && trace == "a")
	trace = ""
	check(tupleOne(true) == IntResult.Err(failure{"bad"}) && trace == "a")
	trace = ""
	check(tupleMany(false) == IntResult.Ok(43) && trace == "p")
	trace = ""
	check(tupleMany(true) == IntResult.Err(failure{"pair"}) && trace == "p")
	trace = ""
	check(tupleOnly(false) == IntResult.Ok(1) && trace == "oafter")
	trace = ""
	check(tupleOnly(true) == IntResult.Err(failure{"only"}) && trace == "o")
	trace = ""
	check(tupleAny(true) == AnyResult.Err(failure{"bad"}))
	trace = ""
	check(tupleNamed(true) == IntResult.Err(failure{"bad"}) && trace == "aD")
	trace = ""
	check(tupleNamed(false) == IntResult.Ok(1) && trace == "aD")
	trace = ""
	check(tupleOrder(0) == IntResult.Ok(6) && trace == "xbc")
	trace = ""
	check(tupleOrder(2) == IntResult.Err(failure{"bad"}) && trace == "xb")
	trace = ""
	check(tupleOrder(3) == IntResult.Err(failure{"bad"}) && trace == "xbc")
	trace = ""

	n, err := resultToError(0)
	check(n == 8 && err == nil && trace == "r")
	trace = ""
	n, err = resultToError(1)
	check(n == 0 && err == error(failure{"provided"}))
	n, err = resultToError(2)
	check(n == 0 && err != nil)
	s, n, err := resultToErrorMany(1)
	check(s == "" && n == 0 && err != nil)
	check(resultToErrorOnly(0) == nil && resultToErrorOnly(1) != nil && resultToErrorOnly(2) != nil)
	trace = ""
	n, err = resultToErrorNamed(1)
	check(n == 0 && err != nil && trace == "rD")
	trace = ""
	n, err = resultToErrorNamed(0)
	check(n == 7 && err == nil && trace == "rD")
	_, err = resultToErrorCoded(true)
	check(err != nil) // A nil *coded is a non-nil error.
	_, err = resultToErrorCoded(false)
	check(err != nil)
	_, err = resultToErrorInterface(true)
	check(err != nil)
	_, err = resultToErrorInterface(false)
	check(err == error(failure{"iface"}))
	println("PASS")
}
