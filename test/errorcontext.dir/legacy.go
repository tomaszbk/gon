package main

import "slices"

func primary(fail bool) (number int, err error) {
	number = 88
	defer func() { observe(number, "", err) }()
	n, problem := read(fail)
	if problem != nil {
		return 0, wrap(problem)
	}
	number = n
	return number, nil
}
func multiple(fail bool) (number int, text string, err error) {
	number, text = 88, "before"
	defer func() { observe(number, text, err) }()
	n, s, problem := many(fail)
	if problem != nil {
		return 0, "", wrap(problem)
	}
	number, text = n, s
	return number, text, nil
}
func args(fail bool) (int, error) {
	fn := argumentFunc()
	a, err := read(false)
	if err != nil {
		return 0, wrap(err)
	}
	b, err := read(fail)
	if err != nil {
		return 0, wrap(err)
	}
	return fn(a, b, third()), nil
}
func named(fail bool) (int, error) {
	b, err := read(fail)
	if err != nil {
		return 0, wrap(err)
	}
	a, err := read(false)
	if err != nil {
		return 0, wrap(err)
	}
	return take(a, b), nil
}
func iterator(fail bool) (int, error) {
	sum := 0
	for n := range slices.Values([]int{1, 2, 3}) {
		value, err := read(fail && n == 2)
		if err != nil {
			return 0, wrap(err)
		}
		sum += value
	}
	return sum, nil
}
func errorOnly(fail bool) error {
	if err := only(fail); err != nil {
		return wrap(err)
	}
	return nil
}
func tupleArgument(fail bool) (string, error) {
	n, s, err := many(fail)
	if err != nil {
		return "", wrap(err)
	}
	return consumeMany(n, s), nil
}
func nested(fail bool) (int, error) {
	run := func() (int, error) {
		n, err := read(fail)
		if err != nil {
			return 0, wrap(err)
		}
		return n, nil
	}
	return run()
}
func shadow(fail bool) (int, error) {
	err := 3
	n, problem := read(fail)
	if problem != nil {
		return 0, wrap(problem)
	}
	return n + err, nil
}
func nilContext() (int, error) {
	n, err := read(true)
	if err != nil {
		return 0, forget(err)
	}
	trace = append(trace, "unreachable")
	return n, sentinel
}
func ignoredContext() error {
	if err := only(true); err != nil {
		return sentinel
	}
	return nil
}
