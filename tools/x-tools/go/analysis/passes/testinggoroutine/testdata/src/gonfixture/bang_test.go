package gon

import "testing"

func parse() (int, error)       { return 1, nil }
func helper(t *testing.T) error { parse()!; return nil }
func TestBang(t *testing.T) {
	go func(u *testing.T) { _ = helper(u) }(t)
	go func(u *testing.T) { _ = func() error { parse() or err => err; return nil }() }(t)
	t.Run("subtest", func(u *testing.T) { _ = helper(u) })
	t.Run("lambda", (u) => { _ = helper(u) })
	go func(u *testing.T) {
		parse() or err {
			u.Fatal(err) // want "call to .*Fatal from a non-test goroutine"
			return
		}
	}(t)
}
