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
