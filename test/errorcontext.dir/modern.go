package main

import "slices"

func primary(fail bool) (number int, err error) {
	number = 88
	defer func() { observe(number, "", err) }()
	number = read(fail) or err => wrap(err)
	return number, nil
}
func multiple(fail bool) (number int, text string, err error) {
	number, text = 88, "before"
	defer func() { observe(number, text, err) }()
	number, text = many(fail) or err => wrap(err)
	return number, text, nil
}
func args(fail bool) (int, error) {
	return argumentFunc()(read(false) or err => wrap(err), read(fail) or err => wrap(err), third()), nil
}
func named(fail bool) (int, error) {
	return take(b: read(fail) or err => wrap(err), a: read(false) or err => wrap(err)), nil
}
func iterator(fail bool) (int, error) {
	sum := 0
	for n := range slices.Values([]int{1, 2, 3}) {
		sum += read(fail && n == 2) or err => wrap(err)
	}
	return sum, nil
}
func errorOnly(fail bool) error { only(fail) or err => wrap(err); return nil }
func tupleArgument(fail bool) (string, error) {
	return consumeMany(many(fail) or err => wrap(err)), nil
}
func nested(fail bool) (int, error) {
	var run func() (int, error) = () => {
		return read(fail) or err => wrap(err), nil
	}
	return run()
}
func shadow(fail bool) (int, error) {
	err := 3
	n := read(fail) or err => wrap(err)
	return n + err, nil
}
func nilContext() (int, error) {
	n := read(true) or err => forget(err)
	trace = append(trace, "unreachable")
	return n, sentinel
}
func ignoredContext() error { only(true) or _ => sentinel; return nil }
