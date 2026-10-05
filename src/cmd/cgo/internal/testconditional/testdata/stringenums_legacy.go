package main

/*
#define teacher_text "teacher"
#define student_text "student"
*/
import "C"

type Role struct {
	kind int
	text string
}

func parseRole(text string) Role {
	switch text {
	case C.teacher_text:
		return Role{kind: 1}
	case C.student_text:
		return Role{kind: 2}
	default:
		return Role{text: text}
	}
}

func (role Role) String() string {
	switch role.kind {
	case 1:
		return C.teacher_text
	case 2:
		return C.student_text
	default:
		return role.text
	}
}

func (role Role) MarshalText() ([]byte, error) { return []byte(role.String()), nil }
func (role *Role) UnmarshalText(text []byte) error {
	*role = parseRole(string(text))
	return nil
}

func parser() func(string) Role { return parseRole }
func known(role Role) bool      { return role.kind != 0 }
