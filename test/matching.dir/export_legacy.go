package lib

type State struct{ kind, value int }

func Count(n int) State { return State{kind: 1, value: n} }
func Size(s State) int {
	if s.kind == 1 {
		return s.value
	}
	return 0
}

type Maybe[T any] struct {
	some  bool
	value T
}

func Wrap[T any](v T) Maybe[T] { return Maybe[T]{some: true, value: v} }
func Empty[T any]() Maybe[T]   { return Maybe[T]{} }
func Get[T any](v Maybe[T], fallback T) T {
	if v.some {
		return v.value
	}
	return fallback
}
