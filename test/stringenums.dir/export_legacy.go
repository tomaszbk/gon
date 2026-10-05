package lib

type Role struct {
	kind int
	text string
}

type Alias = Role

type Generic[T any] struct{ text string }

func ParseGeneric[T any](text string) Generic[T]      { return Generic[T]{text} }
func (value Generic[T]) String() string               { return value.text }
func (value Generic[T]) MarshalText() ([]byte, error) { return []byte(value.String()), nil }
func (value *Generic[T]) UnmarshalText(data []byte) error {
	*value = ParseGeneric[T](string(data))
	return nil
}

var Parser = ParseRole

func ParseRole(text string) Role {
	switch text {
	case "teacher":
		return Role{kind: 1}
	case "student":
		return Role{kind: 2}
	default:
		return Role{text: text}
	}
}

func (role Role) String() string {
	switch role.kind {
	case 0:
		return role.text
	case 1:
		return "teacher"
	case 2:
		return "student"
	}
	panic("invalid role tag")
}

func (role Role) MarshalText() ([]byte, error) { return []byte(role.String()), nil }
func (role *Role) UnmarshalText(data []byte) error {
	*role = ParseRole(string(data))
	return nil
}

func Describe(role Role) string {
	switch role.kind {
	case 0:
		return "unknown:" + role.text
	case 1:
		return "teacher"
	case 2:
		return "student"
	}
	panic("invalid role tag")
}
