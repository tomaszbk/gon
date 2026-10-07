package gonfixture

import (
	"fmt"
	"sync"
)

func interpolation() {
	var lock sync.Mutex
	_ = $"${lock}" // want "string interpolation copies lock value: sync.Mutex"
	_ = $"${&lock}"
	var stored struct{ Lock sync.Mutex }
	_ = $"${stored}" // want "string interpolation copies lock value: struct.* contains sync.Mutex"
	_ = $"${new(sync.Mutex)}"
}
