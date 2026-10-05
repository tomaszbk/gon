package lib

type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

type Alias = Role

type Generic[T any] enum string {
	default Other(string)
	Ready = "ready"
}

var Parser = Alias.Parse
