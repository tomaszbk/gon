package main

import (
	"featuretest/lib"
	"fmt"
)

func main() {
	if lib.MakeBox(7) != lib.MakeBox(7) {
		panic("exported generic enum")
	}
	if lib.MakeOption(0) == lib.EmptyOption[int]() {
		panic("exported canonical Option")
	}
	if lib.Failed() == lib.Success() {
		panic("exported canonical Result")
	}
	constructor := lib.Constructor()
	if constructor("value") != lib.MakeBox("value") {
		panic("exported constructor callback")
	}
	if lib.RecordValue() != lib.RecordValue() {
		panic("exported record enum")
	}
	fmt.Println("enum exports ok")
}
