package lib

type Failure enum {
	default Missing
	Coded(string)
}

func (Failure) Error() string { return "failure" }

func NewMissing() error       { return Failure.Missing }
func NewCoded(c string) error { return Failure.Coded(c) }

func Code(err error) string {
	return switch err {
	case Failure.Coded(code) => code
	case Failure.Missing => "missing"
	default => "other"
	}
}

type Box[T any] enum {
	default Empty
	Full(T)
}

func (Box[T]) Error() string { return "box" }

func Full[T any](v T) error { return Box[T].Full(v) }

func Unbox[T any](err error, fallback T) T {
	return switch err {
	case Box[T].Full(value) => value
	default => fallback
	}
}
