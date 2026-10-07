package gon
func f() {}
func g(h func()) {
 f?() // want "nil test of function f is redundant"
 _ = f ?? h // want "nil test of function f is redundant"
 h?()
 _ = h ?? f
 _ = f == nil // want "comparison of function f == nil is always false"
}
