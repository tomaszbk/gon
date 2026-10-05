package main

// Go error tuples and Result converted by hand: the legacy spelling of
// postfix ! across both boundaries.

type failure struct{ text string }

func (f failure) Error() string { return f.text }

type coded struct{ code int }

func (c *coded) Error() string { return "coded" }

type nilError interface{ Error() string }

type IntResult struct {
	Value   int
	Problem error
	Failed  bool
}
type AnyResult struct {
	Value   int
	Problem any
	Failed  bool
}
type CodedResult struct {
	Value   int
	Problem *coded
	Failed  bool
}
type NilErrorResult struct {
	Value   int
	Problem nilError
	Failed  bool
}

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
		return IntResult{Value: 7}
	case 1:
		return IntResult{Problem: failure{"provided"}, Failed: true}
	}
	return IntResult{Failed: true}
}

// An error tuple fails as Err inside a function returning one Result.
func tupleOne(bad bool) IntResult {
	n, err := source("a", 5, bad)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n * 2}
}
func tupleMany(bad bool) IntResult {
	n, s, err := pair(bad)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n + len(s)}
}
func tupleOnly(bad bool) IntResult {
	if err := only(bad); err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	mark("after")
	return IntResult{Value: 1}
}
func tupleAny(bad bool) AnyResult {
	n, err := source("a", 5, bad)
	if err != nil {
		return AnyResult{Problem: err, Failed: true}
	}
	return AnyResult{Value: n}
}
func tupleNamed(bad bool) (r IntResult) {
	r = IntResult{Value: 100}
	defer func() { mark("D") }()
	n, err := source("a", 1, bad)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n}
}
func tupleOrder(failAt int) IntResult {
	a := 1
	mark("x")
	b, err := source("b", 2, failAt == 2)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	c, err := source("c", 3, failAt == 3)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: a + b + c}
}

// A failed Result fails as its error inside a function returning error last.
func orNilResult(err error) error {
	if err == nil {
		return failure{"nil result"}
	}
	return err
}
func resultToError(kind int) (int, error) {
	r := provide(kind)
	if r.Failed {
		return 0, orNilResult(r.Problem)
	}
	return r.Value + 1, nil
}
func resultToErrorMany(kind int) (string, int, error) {
	r := provide(kind)
	if r.Failed {
		return "", 0, orNilResult(r.Problem)
	}
	return "ok", r.Value, nil
}
func resultToErrorOnly(kind int) error {
	r := provide(kind)
	if r.Failed {
		return orNilResult(r.Problem)
	}
	return nil
}
func resultToErrorNamed(kind int) (n int, err error) {
	n = 99
	defer func() { mark("D") }()
	r := provide(kind)
	if r.Failed {
		return 0, orNilResult(r.Problem)
	}
	return r.Value, nil
}
func resultToErrorCoded(nilPayload bool) (int, error) {
	r := CodedResult{Failed: true}
	if !nilPayload {
		r.Problem = &coded{3}
	}
	if r.Failed {
		var err error = r.Problem
		return 0, err
	}
	return r.Value, nil
}
func resultToErrorInterface(nilPayload bool) (int, error) {
	r := NilErrorResult{Failed: true}
	if !nilPayload {
		r.Problem = failure{"iface"}
	}
	if r.Failed {
		var err error = r.Problem
		return 0, orNilResult(err)
	}
	return r.Value, nil
}

func main() {
	check(tupleOne(false).Value == 10 && trace == "a")
	trace = ""
	r := tupleOne(true)
	check(r.Failed && r.Problem == failure{"bad"} && trace == "a")
	trace = ""
	check(tupleMany(false).Value == 43 && trace == "p")
	trace = ""
	check(tupleMany(true).Failed && trace == "p")
	trace = ""
	check(tupleOnly(false).Value == 1 && trace == "oafter")
	trace = ""
	check(tupleOnly(true).Failed && trace == "o")
	trace = ""
	ar := tupleAny(true)
	check(ar.Failed && ar.Problem == any(failure{"bad"}))
	trace = ""
	check(tupleNamed(true).Failed && trace == "aD")
	trace = ""
	check(!tupleNamed(false).Failed && trace == "aD")
	trace = ""
	check(tupleOrder(0).Value == 6 && trace == "xbc")
	trace = ""
	check(tupleOrder(2).Failed && trace == "xb")
	trace = ""
	check(tupleOrder(3).Failed && trace == "xbc")
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
