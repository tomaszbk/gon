package x

import (
	"errors"
	"strconv"
)

var errBoom = errors.New("boom")

type CodedError struct{ Code int }

func (e *CodedError) Error() string { return "coded:" + strconv.Itoa(e.Code) }

func parse(s string) (int, error) { return strconv.Atoi(s) }
func fail() error                 { return errBoom }
func many() (int, string, error)  { return 1, "two", errBoom }
func pointer() (*int, error)      { return nil, errBoom }
