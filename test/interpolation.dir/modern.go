package main

import alias "fmt"

func propagation(fail bool) (out string, err error) {
	out = "stale"
	defer func() { trace += "D" + out }()
	return $"${mark("A", 1)}:${source(fail)!}:${mark("B", 2):%04d}", nil
}
func scenario() string {
	x := 3
	price := 2.5
	return $"{ } $ 50% ${mark("C", x)} ${price:%.2f} ${call(right: mark("R", 2), left: mark("L", 1))} ${map[string]int{"key": 5}["key"]} ${[]int{1, 2, 3}[1:]} ${if x > 0 { "yes" } else { "no" }} ${$"nested:${x}"} ${mark("I", 11):%[1]d} ${mark("J", 22):%[1]d} \${literal} ${"line\n":%q}"
}
func shadowed() string { alias := 5; return $"shadow:${alias}" }
func multiline() string {
	return $`raw
	{"empty": []} ${"${"} ${3}`
}
func empty() string { return $"" }

func blocks(fail bool) (string, error) {
	return $"${switch fail { case false => 1; case true => 2 }}:${source(fail) or err { return "", err }}:${func() int { var (n = 1; m = 2); type S struct { A int; B int }; _ = S{}; switch n { case 1: n += m; default: n = 0 }; return n }()}", nil
}
func rawBlocks() string {
	return $`raw
${switch true { case true => 1; case false => 2 }} ${func() int { n := 1; switch n { case 1: n++; default: n-- }; return n }()}
end`
}

//line interpolation-generated.go:1
func captured() string {
	offset := 1
	var f func(int) string = (n) => $"${n + offset}"
	offset = 2
	return f(3)
}
