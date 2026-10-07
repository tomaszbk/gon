package main

var marker *int

type record struct{ value *int }

func field(r record) int         { return *r.value }
func pointerField(r *record) int { return *r.value }
func rangeSlice(xs []*int) int {
	for _, p := range xs {
		return *p
	}
	return 0
}
func rangeMap(xs map[string]*int) int {
	for _, p := range xs {
		return *p
	}
	return 0
}
func receive(ch chan *int) int { return *<-ch } // want "dereferenced|accessed field|sliced into"
func typeSwitch(x any) int {
	switch p := x.(type) {
	case *int:
		return *p // want "dereferenced|accessed field|sliced into"
	}
	return 0
}
func (r *record) read() int             { return *r.value } // want "dereferenced|accessed field|sliced into"
func appended(xs []*int, p *int) []*int { return append(xs, p) }
func unsafe() {
	var p *int
	r := record{p}
	_ = field(r)
	_ = pointerField(&r)
	_ = rangeSlice([]*int{p})
	_ = rangeMap(map[string]*int{"p": p})
	ch := make(chan *int, 1)
	ch <- p
	_ = receive(ch)
	_ = typeSwitch(p)
	_ = r.read()
	_ = (*record).read(&r)
	_ = *appended(nil, p)[0] // want "dereferenced|accessed field|sliced into"
}
func nilReceiver() { var r *record; _ = r.read(); _ = (*record).read(r) }

func main() {
	check(func() { _ = field(record{}) })
	check(func() { _ = pointerField(&record{}) })
	check(func() { _ = rangeSlice([]*int{nil}) })
	check(func() { _ = rangeMap(map[string]*int{"nil": nil}) })
	check(func() { ch := make(chan *int, 1); ch <- nil; _ = receive(ch) })
	check(func() { _ = typeSwitch((*int)(nil)) })
	check(func() { var r *record; _ = (*record).read(r) })
	check(func() { _ = *appended(nil, nil)[0] })
	check(closedChannel)
	check(selectedClosedChannel)
	check(helperClosedChannel)
	check(constructorClosedChannel)
	check(drainingReturn)
	check(captureDeferredNil)
	captureConcurrentNil()
	println("nil precision: 15 panics")
}
func check(f func()) {
	panicked := false
	func() { defer func() { panicked = recover() != nil }(); f() }()
	if !panicked {
		panic("missing nil panic")
	}
}

func closedChannel() { ch := make(chan *int); close(ch); _ = *<-ch }

func selectedClosedChannel() {
	ch := make(chan *int, 1)
	ch <- new(int)
	close(ch)
	select {
	case <-ch:
	}
	_ = *<-ch
}
func drainClose(ch chan *int) { <-ch; close(ch) }
func helperClosedChannel() {
	ch := make(chan *int, 1)
	ch <- new(int)
	drainClose(ch)
	_ = *<-ch
}
func constructorClosedChannel() {
	ch := makeClosedChannel()
	_ = *<-ch
}
func makeClosedChannel() chan *int {
	ch := make(chan *int)
	close(ch)
	return ch
}
func drainThenReturn(ch chan *int) *int {
	<-ch
	close(ch)
	return <-ch
}
func drainingReturn() {
	ch := make(chan *int, 1)
	ch <- new(int)
	_ = *drainThenReturn(ch)
}

func captureDeferredNil() {
	p := new(int)
	read := func() { _ = *p }
	defer read()
	p = nil
}
func captureConcurrentNil() {
	p := new(int)
	gate := make(chan struct{})
	done := make(chan bool, 1)
	read := func() {
		defer func() { done <- recover() != nil }()
		<-gate
		_ = *p
	}
	go read()
	p = nil
	close(gate)
	if !<-done {
		panic("missing captured nil panic")
	}
	p = new(int)
}
