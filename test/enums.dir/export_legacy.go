package lib

type Box[T any] struct {
	Present bool
	Value   T
}
type Record struct {
	Present bool
	Name    string
	Count   int
}
type Optional[T any] struct {
	Present bool
	Value   T
}
type Outcome[T any, E any] struct {
	Failed bool
	Value  T
	Error  E
}

func MakeBox[T any](value T) Box[T]         { return Box[T]{true, value} }
func MakeOption[T any](value T) Optional[T] { return Optional[T]{true, value} }
func EmptyOption[T any]() Optional[T]       { return Optional[T]{} }
func Failed() Outcome[int, error]           { return Outcome[int, error]{Failed: true} }
func Success() Outcome[int, error]          { return Outcome[int, error]{} }
func Constructor() func(string) Box[string] { return MakeBox[string] }
func RecordValue() Record                   { return Record{true, "record", 3} }
