package misc

import (
	"strings"
	"testing"

	. "golang.org/x/tools/gopls/internal/test/integration"
)

func TestGonOptionalHover(t *testing.T) {
	const source = `
-- go.mod --
module example.com/optionalhover

go 1.27
-- p.go --
package p

import "unsafe"

type Pointer *int
type MaybePointer = (*int)?
type Maybe[T any] = T?

var pointer (*int)?
var iface any?
var slice ([]int)?
var mapping (map[string]int)?
var callback (func())?
var channel (chan int)?
var unsafePointer unsafe.Pointer?
var namedPointer Pointer?
var aliased MaybePointer
var number int?
var nested (int?)?
var plainPointer *int
var plainNumber int

type Holder struct { Value (*int)? }
var holder Holder
var items [](*int)?

func generic[T any](value Maybe[T]) {
	_ = value
}

func load(param (*int)?) (*int)? {
	_ = pointer
	_ = iface
	_ = slice
	_ = mapping
	_ = callback
	_ = channel
	_ = unsafePointer
	_ = namedPointer
	_ = aliased
	_ = number
	_ = nested
	_ = plainPointer
	_ = plainNumber
	_ = holder.Value
	_ = param
	return items[0]
}
`
	WithOptions(Modes(Default)).Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("p.go")
		const nilNote = "Presence does not imply a non-nil payload: a typed nil value is present."
		for _, at := range []string{
			`_ = (pointer)`, `_ = (iface)`, `_ = (slice)`, `_ = (mapping)`,
			`_ = (callback)`, `_ = (channel)`, `_ = (unsafePointer)`,
			`_ = (namedPointer)`, `_ = (aliased)`, `holder\.(Value)`,
			`_ = (param)`, `type (MaybePointer)`, `return items(\[)`,
		} {
			hover, _ := env.Hover(env.RegexpSearch("p.go", at))
			if hover == nil || !strings.Contains(hover.Value, nilNote) {
				t.Errorf("optional nilable hover at %s: %+v", at, hover)
			}
		}
		hover, _ := env.Hover(env.RegexpSearch("p.go", `_ = (number)`))
		if hover == nil || !strings.Contains(hover.Value, "including its zero value") || strings.Contains(hover.Value, "nil payload") {
			t.Errorf("optional scalar hover: %+v", hover)
		}
		hover, _ = env.Hover(env.RegexpSearch("p.go", `_ = (nested)`))
		if hover == nil || !strings.Contains(hover.Value, "the inner optional may be absent") {
			t.Errorf("nested optional hover: %+v", hover)
		}
		hover, _ = env.Hover(env.RegexpSearch("p.go", `_ = (value)`))
		if hover == nil || !strings.Contains(hover.Value, "For nilable payload types, presence does not imply a non-nil payload") {
			t.Errorf("generic optional hover: %+v", hover)
		}
		for _, at := range []string{`_ = (plainPointer)`, `_ = (plainNumber)`} {
			hover, _ := env.Hover(env.RegexpSearch("p.go", at))
			if hover == nil || strings.Contains(hover.Value, "Optional values") {
				t.Errorf("ordinary Go hover at %s: %+v", at, hover)
			}
		}
	})
}
