package gonmemory

import (
	"encoding/json"
	"gon.test/gonapi"
)

func clear(pointer **int) { *pointer = nil }
func mutation() {
	p := if true { new(int) } else { new(int) }
	clear(&p)
	_ = *p // want "dereferenced"
}
func capture() {
	p := new(int)
	var f func() int = () => *p // want "dereferenced"
	p = nil
	_ = f()
}
func safe() {
	p := if true { new(int) } else { new(int) }
	m := map[string]*int{"k": p}
	xs := []*int{p}
	_ = *m["k"]
	_ = *xs[0]
	var f func() int = () => *p
	_ = f()
}
func nilElements() {
	m := map[string]*int{"k": nil}
	xs := []*int{nil}
	_ = *m["k"] // want "dereferenced"
	_ = *xs[0]  // want "dereferenced"
}

func missingMapKey() {
	values := map[string]*int{"k": new(int)}
	_ = *values["missing"] // want "dereferenced"
	empty := map[string]*int{}
	_ = *empty["missing"] // want "dereferenced"
}
func safeMapKeys() {
	values := map[string]*int{"safe": new(int), "nil": nil}
	_ = *values["safe"]
	if value, ok := values["missing"]; ok {
		_ = *value
	}
}
func initialize(pointer **int) { *pointer = new(int) }
func safeMutation()            { var pointer *int; initialize(&pointer); _ = *pointer }

type API interface{ Read(value *int) *int }
type Reader struct{}

func (Reader) Read(value *int) *int { return value }
func safeInterface() {
	var api API = Reader{}
	_ = *api.Read(value: new(int))
}
func nilInterfaceArgument() {
	var api API = Reader{}
	_ = *api.Read(value: nil) // want "dereferenced"
}

func mapHelper(values map[string]*int) int { return *values["safe"] }
func sliceHelper(values []*int) int        { return *values[0] }
func safeHelpers() {
	_ = mapHelper(map[string]*int{"safe": new(int)})
	_ = sliceHelper([]*int{new(int)})
}
func badMapHelper(values map[string]*int) int { return *values["missing"] } // want "dereferenced"
func badSliceHelper(values []*int) int        { return *values[0] }         // want "dereferenced"
func unsafeHelpers()                          { _ = badMapHelper(map[string]*int{}); _ = badSliceHelper([]*int{nil}) }

func safeGenericOptional() {
	if value := gonapi.Optional(new(int)); value is p? {
		_ = *p
	}
}
func nilGenericOptional() {
	if value := gonapi.Optional((*int)(nil)); value is p? {
		_ = *p // want "dereferenced"
	}
}

type PointerReceiver struct{ Value int }

func (value *PointerReceiver) Get() int { return value.Value } // want "dereferenced"
func (value *PointerReceiver) SafeGet() int {
	if value == nil {
		return 0
	}
	return value.Value
}
func nilReceiver()     { var value *PointerReceiver; _ = value.Get() }
func safeNilReceiver() { var value *PointerReceiver; _ = value.SafeGet() }

func decodedField(data []byte) error {
	var parsed struct{ Values *[]int }
	json.Unmarshal(data, &parsed)!
	if parsed.Values == nil {
		return nil
	}
	_ = *parsed.Values
	return nil
}

type Failure enum {
	default Internal(error)
	Rejected { Code string }
}

func (Failure) Error() string { return "failure" }
func errorPattern(err error) {
	switch err {
	case Failure.Rejected{Code: code} => {
		_ = code
	}
	default => {
	}
	}
}

func errorPayload() {
	var err error = Failure.Internal(nil)
	switch err {
	case Failure.Internal(cause) => {
		_ = cause.Error() // want "dereferenced"
	}
	default => {
	}
	}
}

func safeOutcome() error {
	value := gonapi.Outcome(false)!
	_ = *value
	return nil
}
func partialOutcome() error {
	value := gonapi.Partial()!
	_ = *value // want "dereferenced"
	return nil
}
