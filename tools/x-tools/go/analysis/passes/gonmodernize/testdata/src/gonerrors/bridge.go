package gonerrors

import (
	"fmt"
	"testing"
)

type bridgeError struct{}

func (bridgeError) Error() string { return "bridge" }

type IntResult = Result[int, error]

func toResult() Result[int, error] {
	// want +1 "replace error check with Gon ! propagation"
	x, err := read()
	if err != nil {
		return .Err(err)
	}
	return .Ok(x)
}

func toResultQualified() Result[int, error] {
	// want +1 "replace error check with Gon ! propagation"
	x, err := read()
	if err != nil {
		return Result[int, error].Err(err)
	}
	return .Ok(x)
}

func toResultAlias() IntResult {
	// want +1 "replace error check with Gon ! propagation"
	x, err := read()
	if err != nil {
		return IntResult.Err(err)
	}
	return .Ok(x)
}

func toResultErrorOnly() Result[string, error] {
	// want +1 "replace error check with Gon ! propagation"
	if err := flush(); err != nil {
		return .Err(err)
	}
	return .Ok("done")
}

func toResultMany() Result[string, error] {
	// want +1 "replace error check with Gon ! propagation"
	x, s, err := many()
	if err != nil {
		return .Err(err)
	}
	return .Ok(fmt.Sprint(x, s))
}

func toResultAny() Result[int, any] {
	// want +1 "replace error check with Gon ! propagation"
	x, err := read()
	if err != nil {
		return .Err(err)
	}
	return .Ok(x)
}

func toResultText() Result[int, string] {
	// want +1 "replace error check with a Gon or handler"
	x, err := read()
	if err != nil {
		return .Err(err.Error())
	}
	return .Ok(x)
}

func toResultConcrete() Result[int, bridgeError] {
	// want +1 "replace error check with a Gon or handler"
	x, err := read()
	if err != nil {
		return .Err(bridgeError{})
	}
	return .Ok(x)
}

func toResultWrapped() Result[int, error] {
	// want +1 "replace error check with a Gon or handler"
	x, err := read()
	if err != nil {
		return .Err(fmt.Errorf("read: %w", err))
	}
	return .Ok(x)
}

func toResultCommented() Result[int, error] {
	// want +1 "replace error check with a Gon or handler"
	x, err := read()
	if err != nil {
		// Keep this context.
		return .Err(err)
	}
	return .Ok(x)
}

func toResultLaterUse() Result[int, error] {
	x, err := read()
	if err != nil {
		return .Err(err)
	}
	consume(err) // err is read again, so the declaration stays
	return .Ok(x)
}

// A function with a further result still uses an ordinary handler.
func toResultAndInt() (Result[int, error], int) {
	// want +1 "replace error check with a Gon or handler"
	x, err := read()
	if err != nil {
		return .Err(err), 0
	}
	return .Ok(x), 1
}

// The test-function rule needs a _test.go file.
func notATestFile(t *testing.T) {
	x, err := read()
	if err != nil {
		t.Fatal(err)
	}
	_ = x
}
