package main

import "fmt"

var effects string

func mark(label string, value int) int { effects += label; return value }
func fallback() int                    { return mark("F", 9) }

type User struct{ Name string }

func (u User) Len() int           { effects += "L"; return len(u.Name) }
func emit(name string, value any) { fmt.Println(name, value, effects); effects = "" }
func check(name string, value, want any, wantEffects string) {
	if value != want || effects != wantEffects {
		panic(fmt.Sprintf("%s: got %v/%q, want %v/%q", name, value, effects, want, wantEffects))
	}
	emit(name, value)
}
func iter(yield func(int) bool) {
	for _, n := range []int{1, 2, 3} {
		if !yield(n) {
			return
		}
	}
}

func panics(f func()) (caught bool) { defer func() { caught = recover() != nil }(); f(); return false }
