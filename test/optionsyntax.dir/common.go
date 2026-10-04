package main

import "fmt"

var effects string

func mark[T any](name string, value T) T { effects += name; return value }
func emit(name string, value any)        { fmt.Println(name, value) }
func main()                              { scenario(); emit("effects", effects) }
