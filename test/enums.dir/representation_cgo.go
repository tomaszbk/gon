package representation

/*
static int twice(int value) { return 2 * value; }
*/
import "C"

type cgoInput enum {
	default Missing
	Value(int)
}
type legacyCgoInput struct {
	present bool
	value   int
}

func cgoModernValue(v cgoInput) int {
	return switch v {
	case cgoInput.Missing => 0
	case cgoInput.Value(value) => int(C.twice(C.int(value)))
	}
}
func cgoTaggedValue(v legacyCgoInput) int {
	if !v.present {
		return 0
	}
	return int(C.twice(C.int(v.value)))
}
