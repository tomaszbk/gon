package gon

import "testing"

func parse() (int, error) { return 1, nil }

func result() Result[int, error] { return .Ok(1) }

func helper(t *testing.T) { parse()! }

func resultHelper(t *testing.T) { result()! }

func TestBang(t *testing.T) {
	go helper(t)       // want "call to .*Fatal from a non-test goroutine \\(helper calls .*Fatal\\)"
	go resultHelper(t) // want "call to .*Fatal from a non-test goroutine \\(resultHelper calls .*Fatal\\)"
	go func(u *testing.T) {
		parse()! // want "call to .*Fatal from a non-test goroutine"
	}(t)
	go func(u *testing.T) {
		result()! // want "call to .*Fatal from a non-test goroutine"
	}(t)
	go (func(u *testing.T) { parse()! })(t) // want "call to .*Fatal from a non-test goroutine"
	t.Run("subtest", func(u *testing.T) { parse()! })
	t.Run("lambda", (u) => { result()! })
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
