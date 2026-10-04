package main

import (
	"fmt"
	"runtime"
	"strings"
)

var effects []string

func text(label string) string      { effects = append(effects, label); return label }
func count(label string, n int) int { effects = append(effects, label); return n }
func assert(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func collect() { runtime.GC(); runtime.GC() }
func finish()  { fmt.Println("enum equivalence:", strings.Join(effects, ",")) }
