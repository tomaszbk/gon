package main

import "reflect"

type Ref struct {
	tag     uint
	text    string
	x, y    int
	hidden  string
	pointer *int
}

func empty() Ref                          { return Ref{} }
func refText(s string) Ref                { return Ref{tag: 1, text: s} }
func refPair(x, y int, hidden string) Ref { return Ref{tag: 2, x: x, y: y, hidden: hidden} }
func refPointer(p *int) Ref               { return Ref{tag: 3, pointer: p} }

type RefSlice struct {
	active bool
	values []int
}

// Existing wrappers may use these spellings without losing reflect.Type.
type reflectionWrapper struct{ reflect.Type }

func (reflectionWrapper) IsEnum(int)       {}
func (reflectionWrapper) EnumVariants(int) {}

var _ reflect.Type = reflectionWrapper{}

func main() {
	var wrapped reflect.Type = reflectionWrapper{reflect.TypeFor[int]()}
	assert(wrapped.Kind() == reflect.Int, "legacy reflect wrapper interface")
	v := refPair(7, 9, "private")
	assert(v.x == 7 && v.y == 9, "active record payload")
	copyValue := v.x
	copyValue = 8
	assert(v.x == 7 && copyValue == 8, "value payload copied")
	assert(reflect.ValueOf(empty()).IsZero() && !reflect.ValueOf(refText("")).IsZero(), "zero policy")
	assert(reflect.DeepEqual(v, refPair(7, 9, "private")) && !reflect.DeepEqual(v, refPair(8, 9, "private")), "reflect equality")
	assert(reflect.DeepEqual(RefSlice{true, []int{1, 2}}, RefSlice{true, []int{1, 2}}) && !reflect.DeepEqual(RefSlice{true, nil}, RefSlice{}), "active deep equality")
	assert(!reflect.ValueOf(RefSlice{}).Comparable(), "whole shape comparability")
	assert(reflect.ValueOf(RefSlice{}).IsZero() && !reflect.ValueOf(RefSlice{true, nil}).IsZero(), "noncomparable shape zero")
	n := 41
	boxed := any(refPointer(&n))
	copy := boxed.(Ref).pointer
	collect()
	assert(copy == &n && *copy == 41, "payload GC retention")
	*copy = 42
	assert(*boxed.(Ref).pointer == 42, "ordinary pointer aliases")
	var dest Ref
	reflect.ValueOf(&dest).Elem().Set(reflect.ValueOf(refText("set")))
	assert(dest == refText("set"), "whole shape copy")
	reflect.ValueOf(&dest).Elem().SetZero()
	assert(dest == empty(), "whole shape zero")
	finish()
}
