package lib

type Role struct {
	kind int
	text string
}

type Alias = Role

type Generic[T any] struct {
	kind int
	text string
}

func ParseGeneric[T any](text string) Generic[T] {
	if text == "ready" {
		return Generic[T]{kind: 1}
	}
	return Generic[T]{text: text}
}
func (value Generic[T]) String() string {
	if value.kind == 1 {
		return "ready"
	}
	return value.text
}
func (value Generic[T]) MarshalText() ([]byte, error) { return []byte(value.String()), nil }
func (value *Generic[T]) UnmarshalText(data []byte) error {
	if string(data) == "ready" {
		*value = Generic[T]{kind: 1}
	} else {
		*value = Generic[T]{text: string(data)}
	}
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
	if string(data) == "teacher" {
		*role = Role{kind: 1}
	} else if string(data) == "student" {
		*role = Role{kind: 2}
	} else {
		*role = Role{text: string(data)}
	}
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
