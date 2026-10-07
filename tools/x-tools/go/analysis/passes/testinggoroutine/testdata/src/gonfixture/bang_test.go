package gon

import "testing"

func parse() (int, error) { return 1, nil }

func helper(t *testing.T) { parse()! }

func TestBang(t *testing.T) {
	go helper(t) // want "call to .*Fatal from a non-test goroutine \\(helper calls .*Fatal\\)"
	go func(u *testing.T) {
		parse() or err => err // want "call to .*Fatal from a non-test goroutine"
	}(t)
	go func(u *testing.T) {
		parse() or err => err // want "call to .*Fatal from a non-test goroutine"
	}(t)
	go (func(u *testing.T) { parse()! })(t) // want "call to .*Fatal from a non-test goroutine"
	t.Run("subtest", func(u *testing.T) { parse()! })
	t.Run("lambda", (u) => { parse()! })
	parse()!
}

// A function that returns error keeps its ordinary propagation: no Fatal.
func returnsError(t *testing.T) error { return nil }

func TestError(t *testing.T) {
	go func(u *testing.T) {
		_ = func() error {
			parse()!
			return nil
		}()
	}(t)
}
