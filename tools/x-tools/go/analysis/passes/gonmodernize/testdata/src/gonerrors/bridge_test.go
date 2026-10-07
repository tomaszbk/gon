package gonerrors

import "testing"

func TestFatal(t *testing.T) {
	// want +1 "replace error check with Gon ! propagation"
	x, err := read()
	if err != nil {
		t.Fatal(err)
	}
	_ = x
	// want +1 "replace error check with Gon ! propagation"
	if err := flush(); err != nil {
		t.Fatal(err)
	}
	// want +1 "replace error check with Gon ! propagation"
	a, s, problem := many()
	if problem != nil {
		t.Fatal(problem)
	}
	_, _ = a, s
}

func newThing(tb testing.TB) int {
	// want +1 "replace error check with Gon ! propagation"
	x, err := read()
	if err != nil {
		tb.Fatal(err)
	}
	return x
}

func BenchmarkFatal(b *testing.B) {
	// want +1 "replace error check with Gon ! propagation"
	if err := flush(); err != nil {
		b.Fatal(err)
	}
}

func FuzzFatal(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		// want +1 "replace error check with Gon ! propagation"
		if err := flush(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSubtest(t *testing.T) {
	t.Run("x", func(u *testing.T) {
		// want +1 "replace error check with Gon ! propagation"
		x, err := read()
		if err != nil {
			u.Fatal(err)
		}
		_ = x
	})
	t.Run("outer", func(u *testing.T) {
		// The handler reports with the outer t, not the closure's first parameter.
		x, err := read()
		if err != nil {
			t.Fatal(err)
		}
		_ = x
	})
}

// Handlers that do more than Fatal(err) keep their meaning.
func TestFatalf(t *testing.T) {
	x, err := read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = x
}

func TestFatalfErrorOnly(t *testing.T) {
	// want +1 "replace error check with a Gon or handler"
	if err := flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
}

func TestErrorThenReturn(t *testing.T) {
	// want +1 "replace error check with a Gon or handler"
	y, err := read()
	if err != nil {
		t.Error(err)
		return
	}
	_ = y
}

func TestLogThenFatal(t *testing.T) {
	z, err := read()
	if err != nil {
		t.Log("note")
		t.Fatal(err)
	}
	_ = z
}

func TestCommented(t *testing.T) {
	// want +1 "replace error check with a Gon or handler"
	if err := flush(); err != nil {
		// Keep this context.
		t.Fatal(err)
	}
}

// A function returning error last keeps the meaning of Fatal: ! would
// return the error instead.
func helperError(t *testing.T) error {
	x, err := read()
	if err != nil {
		t.Fatal(err)
	}
	_ = x
	return nil
}

// An unnamed first parameter cannot report a failure.
func blankFirst(_ *testing.T, t2 *testing.T) {
	x, err := read()
	if err != nil {
		t2.Fatal(err)
	}
	_ = x
}

// The method receiver is not a parameter.
type suite struct{ t *testing.T }

func (s suite) check() {
	x, err := read()
	if err != nil {
		s.t.Fatal(err)
	}
	_ = x
}

func ErrorOnlyInTest(t *testing.T) {
	// want +1 "replace error check with Gon ! propagation"
	if err := flush(); err != nil {
		t.Fatal(err)
	}
}
