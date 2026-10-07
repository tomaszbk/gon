package lib

type Box[T any] enum {
	Full(T)
	default Empty
}
type Record enum {
	Value  {
		Name  string
		Count int
	}
	default Empty
}

func MakeBox[T any](value T) Box[T]         { return Box[T].Full(value) }
func MakeOption[T any](value T) T?   { return (T?)((T)(value)) }
func EmptyOption[T any]() T?         { return (T?)(nil) }
func Constructor() func(string) Box[string] { return Box[string].Full }
func RecordValue() Record                   { return Record.Value{Name: "record", Count: 3} }
