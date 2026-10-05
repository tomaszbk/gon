package main

/*
#define teacher_text "teacher"
#define student_text "student"
*/
import "C"

type Role enum string {
	default Unknown(string)
	Teacher = C.teacher_text
	Student = C.student_text
}

func parser() func(string) Role { return Role.Parse }

func known(role Role) bool {
	return switch role {
	case Role.Unknown(_) => false
	case Role.Teacher => true
	case Role.Student => true
	}
}
