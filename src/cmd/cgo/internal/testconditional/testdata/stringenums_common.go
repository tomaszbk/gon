package main

/*
#include <stdlib.h>
static int call_count;
static const char *role_text(int kind) {
	call_count++;
	switch (kind) {
	case 0: return "";
	case 1: return "teacher";
	case 2: return "student";
	default: return "future";
	}
}
static int calls(void) { return call_count; }
static int text_length(const char *text) {
	int n = 0;
	while (*text++) n++;
	return n;
}
*/
import "C"

import (
	"encoding"
	"encoding/json"
	"fmt"
	"unsafe"
)

var (
	_ fmt.Stringer             = *new(Role)
	_ encoding.TextMarshaler   = *new(Role)
	_ encoding.TextUnmarshaler = (*Role)(nil)
)

func must(ok bool, description string) {
	if !ok {
		panic(description)
	}
}

func main() {
	var zero Role
	must(zero.String() == "" && !known(zero), "zero fallback")
	parse := parser()
	for kind := 0; kind < 4; kind++ {
		before := C.calls()
		role := parse(C.GoString(C.role_text(C.int(kind))))
		must(C.calls() == before+1, "C parser input evaluated once")
		must(known(role) == (kind == 1 || kind == 2), "known and fallback matching")
		text, err := role.MarshalText()
		must(err == nil && string(text) == role.String(), "text interface")
		wire, err := json.Marshal(role)
		must(err == nil, "JSON encoding")
		var decoded Role
		must(json.Unmarshal(wire, &decoded) == nil && decoded == role, "JSON decoding")
		must(json.Unmarshal([]byte("null"), &decoded) == nil && decoded == role, "JSON null retains value")
		cText := C.CString(role.String())
		length := int(C.text_length(cText))
		C.free(unsafe.Pointer(cText))
		must(length == len(role.String()), "explicit string C boundary")
		fmt.Printf("%d:%s:%s:%d\n", kind, role.String(), wire, length)
	}
	var decoded Role
	must(decoded.UnmarshalText([]byte("student")) == nil && known(decoded), "pointer method handles known text")
	must(decoded.UnmarshalText([]byte("future")) == nil && !known(decoded), "pointer method handles unknown text")
	fmt.Println("PASS")
}
