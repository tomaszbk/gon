package main

// One Gon declaration must not change the nilness of ordinary Go operations.
var marker int?

type record struct{ value *int }

func field(r record) int         { return *r.value }
func pointerField(r *record) int { return *r.value }
func rangeSlice(xs []*int) int {
	var sum int
	for _, p := range xs {
		sum += *p
	}
	return sum
}
func rangeMap(xs map[string]*int) int {
	var sum int
	for _, p := range xs {
		sum += *p
	}
	return sum
}
func receive(ch chan *int) int { return *<-ch }
func typeSwitch(x any) int {
	switch p := x.(type) {
	case *int:
		return *p
	}
	return 0
}
func (r *record) read() int             { return *r.value }
func appended(xs []*int, p *int) []*int { return append(xs, p) }
func safe() {
	p := new(int)
	r := record{p}
	_ = field(r: r)
	_ = pointerField(&r)
	_ = rangeSlice([]*int{p})
	_ = rangeMap(map[string]*int{"p": p})
	ch := make(chan *int, 1)
	ch <- p
	_ = receive(ch)
	_ = typeSwitch(x: p)
	_ = r.read()
	_ = (*record).read(&r)
	_ = *appended(nil, p)[0]
	_ = tupleSlice()
	checkedClosedChannel()
	bufferedClosedChannel()
	closeAfterReceive()
	captureInitialized(false)
	captureInitialized(true)
	if captureDeferred(false) != 7 || captureDeferred(true) != 0 || captureDeferredInitialization() != 7 {
		panic("deferred capture changed")
	}
}

func pointers() ([]*int, error) { return []*int{new(int)}, nil }
func tupleSlice() error {
	xs := pointers()!
	_ = *xs[0]
	return nil
}

func checkedClosedChannel() {
	ch := make(chan *int, 1)
	ch <- new(int)
	close(ch)
	if p, ok := <-ch; ok {
		_ = *p
	}
}

func bufferedClosedChannel() {
	ch := make(chan *int, 1)
	ch <- new(int)
	close(ch)
	_ = *<-ch
}

func closeAfterReceive() {
	ch := make(chan *int, 1)
	ch <- new(int)
	p := <-ch
	close(ch)
	_ = *p
}

func main() { safe(); println("nil precision: safe") }

func captureInitialized(skip bool) {
	if skip {
		return
	}
	p := new(int)
	*p = 7
	done := make(chan int, 1)
	var read func() = () => { done <- *p }
	go read()
	if value := <-done; value != 7 {
		panic("capture changed")
	}
}

func captureDeferred(skip bool) (value int) {
	if skip {
		return
	}
	p := new(int)
	*p = 7
	var read func() = () => { value = *p }
	defer read()
	return
}

func captureDeferredInitialization() (value int) {
	var p *int
	var read func() = () => { value = *p }
	defer read()
	p = new(int)
	*p = 7
	return
}
