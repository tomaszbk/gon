package representation

import (
	"runtime"
	"testing"
	"unsafe"
)

type legacyOption[T any] struct {
	present bool
	value   T
}
type Unit enum {
	default Red
	Blue
}
type legacyUnit struct{ tag uint }

type Payload enum {
	default Empty
	Integer(int)
	Pointer(*int)
}
type legacyPayload struct {
	tag     uint
	integer int
	pointer *int
}

var sink int
var optionSink int?
var oldOptionSink legacyOption[int]
var payloadSink Payload
var oldPayloadSink legacyPayload

func TestAlternativeRepresentation(t *testing.T) {
	t.Logf("sizes (bytes): int?=%d tagged=%d; Payload=%d tagged=%d; (*int)?=%d tagged=%d", unsafe.Sizeof(optionSink), unsafe.Sizeof(oldOptionSink), unsafe.Sizeof(payloadSink), unsafe.Sizeof(oldPayloadSink), unsafe.Sizeof(((*int)?)(nil)), unsafe.Sizeof(legacyOption[*int]{}))
	t.Logf("unit and zero-size payload (bytes): Unit=%d tagged=%d; Option[struct{}]=%d tagged=%d", unsafe.Sizeof(Unit.Red), unsafe.Sizeof(legacyUnit{}), unsafe.Sizeof((struct{}?)(nil)), unsafe.Sizeof(legacyOption[struct{}]{}))
	modern := testing.AllocsPerRun(1000, func() {
		optionSink = (int?)((int)(42))
		payloadSink = Payload.Integer(42)
	})
	legacy := testing.AllocsPerRun(1000, func() {
		oldOptionSink = legacyOption[int]{true, 42}
		oldPayloadSink = legacyPayload{tag: 1, integer: 42}
	})
	t.Logf("construct allocations/run: modern=%g tagged=%g", modern, legacy)
	var start, end runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&start)
	values := make([](*int)?, 10000)
	for i := range values {
		n := i
		values[i] = ((*int)?)((*int)(&n))
	}
	runtime.GC()
	runtime.ReadMemStats(&end)
	sum := 0
	for _, v := range values {
		sum += *(v ?? nil)
	}
	if sum != 49995000 {
		t.Fatal("GC lost active pointers")
	}
	t.Logf("10k retained pointer Options: heap delta=%d bytes, GC cycles=%d", int64(end.HeapAlloc)-int64(start.HeapAlloc), end.NumGC-start.NumGC)
	runtime.KeepAlive(values)
	runtime.GC()
	runtime.ReadMemStats(&start)
	oldValues := make([]legacyOption[*int], 10000)
	for i := range oldValues {
		n := i
		oldValues[i] = legacyOption[*int]{true, &n}
	}
	runtime.GC()
	runtime.ReadMemStats(&end)
	sum = 0
	for _, v := range oldValues {
		sum += *v.value
	}
	if sum != 49995000 {
		t.Fatal("GC lost tagged pointers")
	}
	t.Logf("10k retained pointer tagged: heap delta=%d bytes, GC cycles=%d", int64(end.HeapAlloc)-int64(start.HeapAlloc), end.NumGC-start.NumGC)
	runtime.KeepAlive(oldValues)
}

func BenchmarkAlternatives(b *testing.B) {
	b.Run("OptionModern", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			optionSink = (int?)((int)(i))
		}
		runtime.KeepAlive(optionSink)
	})
	b.Run("OptionTagged", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			oldOptionSink = legacyOption[int]{true, i}
		}
		runtime.KeepAlive(oldOptionSink)
	})
	b.Run("EnumModern", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			payloadSink = Payload.Integer(i)
		}
		runtime.KeepAlive(payloadSink)
	})
	b.Run("EnumTagged", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			oldPayloadSink = legacyPayload{tag: 1, integer: i}
		}
		runtime.KeepAlive(oldPayloadSink)
	})
	b.Run("MatchModern", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sink = switch payloadSink {
			case Payload.Empty => 0
			case Payload.Integer(n) => n
			case Payload.Pointer(p) => *p
			}
		}
	})
	b.Run("MatchTagged", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			switch oldPayloadSink.tag {
			case 0:
				sink = 0
			case 1:
				sink = oldPayloadSink.integer
			case 2:
				sink = *oldPayloadSink.pointer
			}
		}
	})
}
