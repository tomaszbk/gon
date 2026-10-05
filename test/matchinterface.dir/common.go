package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

var io_EOF = io.EOF

var trace []string

func mark(s string) { trace = append(trace, s) }
func expect(s string, got, want any) {
	if got != want {
		panic(fmt.Sprintf("%s got %v want %v", s, got, want))
	}
	fmt.Println(s, got)
}

// shim reports a different error through its As method. The method records
// every call, so the traces of the paired programs show how often and in what
// order the error tree is searched.
type shim struct{ name, as string }

func (s shim) Error() string { return "shim:" + s.name }
func (s shim) As(target any) bool {
	mark(fmt.Sprintf("as:%s:%T", s.name, target))
	switch t := target.(type) {
	case *dbError:
		if s.as == "db" {
			*t = newNotFound(s.name)
			return true
		}
	case *apiError:
		if s.as == "api" {
			*t = newRetry(7, s.name)
			return true
		}
	}
	return false
}

// multi unwraps to several errors, some of them nil.
type multi []error

func (m multi) Error() string   { return "multi" }
func (m multi) Unwrap() []error { return m }

// chain unwraps to a single error and may end the chain with nil.
type chain struct{ next error }

func (c chain) Error() string { return "chain" }
func (c chain) Unwrap() error { return c.next }

func observed(counter *int, err error) error {
	*counter++
	mark("subject")
	return err
}

type shape interface{ Area() int }

func ptrTo[T any](v T) *T { return &v }

type plain struct{ n int }

func (p plain) Area() int { return p.n }

type pointerShape struct{}

func (*pointerShape) Area() int { return 99 }

func main() {
	notFound := newNotFound("users")
	conflict := newConflict("orders", 0)
	expect("classify/db", classify(notFound), "db-not-found:users")
	expect("classify/wrapped", classify(fmt.Errorf("load: %w", newRejected(503, "busy", "later"))), "server:503:busy")
	expect("classify/rejected", classify(newRejected(400, "bad", "no")), "rejected:400:no")
	expect("classify/join", classify(errors.Join(errors.New("a"), fmt.Errorf("b: %w", newMissing()))), "missing")
	expect("classify/multi-wrap", classify(fmt.Errorf("%w and %w", errors.New("x"), newRetry(2, "again"))), "retry:2:again")
	expect("classify/multi-nil", classify(multi{nil, errors.New("x"), nil, newConflict("t", 5)}), "conflict:t:5")
	expect("classify/chain-end", classify(chain{next: chain{}}), "other")
	expect("classify/nested", classify(newInternal(newNotFound("inner"))), "internal-not-found:inner")
	expect("classify/nested-other", classify(newInternal(errors.New("boom"))), "internal")
	expect("classify/internal-nil", classify(newInternal(nil)), "internal")
	expect("classify/zero-key", classify(conflict), "conflict-zero:orders")
	expect("classify/guard-outer", classify(fmt.Errorf("w: %w", io_EOF)), "eof")
	expect("classify/nil", classify(nil), "other")
	expect("classify/foreign", classify(errors.New("foreign")), "other")
	var zeroAPI apiError
	var zeroDB dbError
	expect("classify/default-values", classify(zeroAPI), "internal")
	expect("classify/default-db", classify(zeroDB), "db-unknown")
	expect("classify/shim-db", classify(shim{"s1", "db"}), "db-not-found:s1")
	expect("classify/shim-api", classify(shim{"s2", "api"}), "retry:7:s2")
	expect("classify/shim-none", classify(shim{"s3", ""}), "other")
	expect("classify/shim-wrapped", classify(fmt.Errorf("w: %w", shim{"s4", "db"})), "db-not-found:s4")

	expect("status/rejected", statusOf(newRejected(418, "tea", "pot")), 418)
	expect("status/missing", statusOf(newMissing()), 404)
	expect("status/not-found", statusOf(fmt.Errorf("w: %w", notFound)), 404)
	expect("status/nil", statusOf(nil), 0)
	expect("status/first-wins", statusOf(errors.Join(newRetry(1, "r"), newMissing())), 500)
	expect("status/first-wins-2", statusOf(errors.Join(newMissing(), newRetry(1, "r"))), 404)
	expect("status/other", statusOf(errors.New("x")), 500)

	var count int
	expect("once/missing", onceKind(&count, newMissing()), "missing")
	expect("once/count-1", count, 1)
	count = 0
	expect("once/retry", onceKind(&count, newRetry(3, "z")), "retry")
	expect("once/count-2", count, 1)
	count = 0
	expect("once/other", onceKind(&count, errors.New("x")), "other")
	expect("once/count-3", count, 1)

	expect("escape", escaped(fmt.Errorf("w: %w", newRejected(1, "c", "m")))(), "m:1")
	expect("escape-none", escaped(errors.New("x"))(), "none")

	expect("shape/circle", shapeName(newRadius(3)), "circle:3")
	expect("shape/big", shapeName(newRadius(30)), "big-circle")
	expect("shape/dot", shapeName(newDot()), "dot")
	expect("shape/square", shapeName(newSide(2)), "side:2")
	expect("shape/unit", shapeName(newUnit()), "unit-square")
	expect("shape/plain", shapeName(plain{4}), "other-shape")
	expect("shape/pointer", shapeName(&pointerShape{}), "other-shape")
	expect("shape/pointer-to-enum", shapeName(ptrTo(newRadius(1))), "other-shape")
	expect("shape/nil", shapeName(nil), "nil-shape")
	expect("any/enum", anyKind(newSide(5)), "square:5")
	expect("any/int", anyKind(5), "other")
	expect("any/nil", anyKind(nil), "other")

	expect("generic/int", boxKind[int](fmt.Errorf("w: %w", newFull(5))), "full:5")
	expect("generic/string", boxKind[string](newFull("s")), "full:s")
	expect("generic/mismatch", boxKind[string](newFull(5)), "none")
	expect("generic/empty", boxKind[int](newEmptyBox[int]()), "empty")
	expect("generic/nil", boxKind[int](nil), "none")

	fmt.Println("trace", strings.Join(trace, "/"))
}
