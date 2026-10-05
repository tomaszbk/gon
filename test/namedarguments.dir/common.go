package main

import "fmt"

var events string

func mark(name string, value int) int          { events += name; return value }
func pair(first, second int) int               { return first*10 + second }
func generic[T ~int](first T, second []T) T    { return first + second[0] }
func callback(first int, fn func(int) int) int { return fn(first) }
func getFunction() func(first, second int) int { events += "F"; return pair }

type receiver int

func getReceiver() receiver                    { events += "R"; return 3 }
func (r receiver) Apply(first, second int) int { return int(r)*100 + pair(first, second) }

type applier interface{ Apply(left, right int) int }

func variadic(prefix int, values ...int) int {
	if len(values) > 0 {
		values[0] += prefix
		return values[0]
	}
	return prefix
}
func values() (int, int)             { return 4, 5 }
func report(label string, value any) { fmt.Println(label, value, events); events = "" }

// Untyped boolean, numeric, rune, string and nil arguments passed by name.
type flag bool

func require(valid bool, status int, code, message string) error {
	events += "q"
	if valid {
		return nil
	}
	return fmt.Errorf("%d %s %s", status, code, message)
}
func affected(n int) (int, error) {
	events += "a"
	if n < 0 {
		return 0, fmt.Errorf("negative %d", n)
	}
	return n, nil
}
func markBool(name string, value bool) bool { events += name; return value }
func describe(ok bool, label string, count int) string {
	return fmt.Sprint(ok, " ", label, " ", count)
}
func tagged(ok flag, label string) string { return fmt.Sprint("tagged ", ok, " ", label) }
func anything(first, second any) string {
	return fmt.Sprintf("%v:%T %v:%T", first, first, second, second)
}
func (r receiver) Valid(ok bool, label string) string {
	return fmt.Sprint("valid ", int(r), " ", ok, " ", label)
}

type validator interface {
	Valid(ok bool, label string) string
}

func choose[T any](ok bool, value T) string { return fmt.Sprint("choose ", ok, " ", value) }
func listed(ok bool, extra ...int) string   { return fmt.Sprint("listed ", ok, " ", extra) }
func constants(small int8, ch rune, name string, ratio float64, wide uint64, text any, ptr *int, err error) string {
	return fmt.Sprint(small, " ", ch, " ", name, " ", ratio, " ", wide, " ", text, " ", ptr == nil, " ", err == nil)
}
func record(ok bool, label string)            { events += fmt.Sprint("[", label, " ", ok, "]") }
func apply(value int, fn func(int) bool) bool { return fn(value) }
