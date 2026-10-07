package main

import (
	"errors"
	"fmt"
	"reflect"
)

var sentinel = errors.New("boom")
var trace []string

func read(fail bool) (int, error) {
	trace = append(trace, fmt.Sprint("read:", fail))
	if fail {
		return 99, sentinel
	}
	return 7, nil
}
func many(fail bool) (int, string, error) {
	trace = append(trace, "many")
	if fail {
		return 99, "partial", sentinel
	}
	return 7, "ok", nil
}
func only(fail bool) error { _, err := read(fail); return err }
func wrap(err error) error {
	trace = append(trace, "wrap")
	return fmt.Errorf("context: %w", err)
}
func forget(err error) error { trace = append(trace, "forget"); return nil }
func take(a, b int) int      { trace = append(trace, "take"); return a*10 + b }
func consumeMany(n int, text string) string {
	trace = append(trace, "consume")
	return fmt.Sprint(n, text)
}
func third() int { trace = append(trace, "third"); return 4 }
func argumentFunc() func(int, int, int) int {
	trace = append(trace, "function")
	return func(a, b, c int) int { trace = append(trace, "arguments"); return a + b + c }
}
func observe(n int, text string, err error) {
	trace = append(trace, fmt.Sprintf("defer:%d:%s:%v", n, text, errors.Is(err, sentinel)))
}
func equal(got, want any) {
	if !reflect.DeepEqual(got, want) {
		panic(fmt.Sprintf("got %#v, want %#v", got, want))
	}
}
func check(name string, fail bool, call func(bool) (int, error), want int, events []string) {
	trace = nil
	n, err := call(fail)
	if n != want || errors.Is(err, sentinel) != fail {
		panic(fmt.Sprintf("%s fail=%v: %d, %v", name, fail, n, err))
	}
	equal(trace, events)
	fmt.Println(name, fail, n, err)
}
func main() {
	for _, fail := range []bool{false, true} {
		if fail {
			check("primary", fail, primary, 0, []string{"read:true", "wrap", "defer:0::true"})
			check("args", fail, args, 0, []string{"function", "read:false", "read:true", "wrap"})
			check("named", fail, named, 0, []string{"read:true", "wrap"})
			check("iterator", fail, iterator, 0, []string{"read:false", "read:true", "wrap"})
		} else {
			check("primary", fail, primary, 7, []string{"read:false", "defer:7::false"})
			check("args", fail, args, 18, []string{"function", "read:false", "read:false", "third", "arguments"})
			check("named", fail, named, 77, []string{"read:false", "read:false", "take"})
			check("iterator", fail, iterator, 21, []string{"read:false", "read:false", "read:false"})
		}
		trace = nil
		n, text, err := multiple(fail)
		if fail {
			if n != 0 || text != "" || !errors.Is(err, sentinel) {
				panic("multiple failure")
			}
			equal(trace, []string{"many", "wrap", "defer:0::true"})
		} else {
			if n != 7 || text != "ok" || err != nil {
				panic("multiple success")
			}
			equal(trace, []string{"many", "defer:7:ok:false"})
		}
		trace = nil
		if errors.Is(errorOnly(fail), sentinel) != fail {
			panic("error only")
		}
		trace = nil
		if n, err := tupleArgument(fail); (err != nil) != fail || n != map[bool]string{false: "7ok", true: ""}[fail] {
			panic("tuple argument")
		}
		trace = nil
		if n, err := nested(fail); (err != nil) != fail || n != map[bool]int{false: 7, true: 0}[fail] {
			panic("nested boundary")
		}
		trace = nil
		if n, err := shadow(fail); (err != nil) != fail || n != map[bool]int{false: 10, true: 0}[fail] {
			panic("shadowed binding")
		}
	}
	trace = nil
	if n, err := nilContext(); n != 0 || err != nil {
		panic("nil context")
	}
	equal(trace, []string{"read:true", "forget"})
	trace = nil
	if err := ignoredContext(); err != sentinel {
		panic("blank binding")
	}
	equal(trace, []string{"read:true"})
	fmt.Println("PASS")
}
