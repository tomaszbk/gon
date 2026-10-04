package lib

type State enum {
	default Empty
	Count(int)
}

func Count(n int) State { return State.Count(n) }
func Size(s State) int {
	return switch s {
	case State.Empty => 0
	case State.Count(n) => n
	}
}

type Maybe[T any] enum {
	default None
	Some(T)
}

func Wrap[T any](v T) Maybe[T] { return Maybe[T].Some(v) }
func Empty[T any]() Maybe[T]   { return Maybe[T].None }
func Get[T any](v Maybe[T], fallback T) T {
	return switch v {
	case Maybe[T].None => fallback
	case Maybe[T].Some(value) => value
	}
}
