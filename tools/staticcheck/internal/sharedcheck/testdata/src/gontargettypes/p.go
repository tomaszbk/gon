package gontargettypes

type node struct{ P *int }

func f(flag bool, p *int, n *node) {
	var callback func(int) int = (x) => x + 1
	var conditional any = if flag { p } else { nil }
	var chain any = n?.P
	var fallback any = p ?? new(int)
	var legacy int = 1 // want "should omit type int"
	_, _, _, _, _ = callback, conditional, chain, fallback, legacy
}

func identityOption(value int?) int? { return value }

func simplified(flag bool, pointer *int) {
	var absent int? = .None
	var explicit (*int)? = .Some(nil)
	var present int? = (.Some(1))
	var failure Result[int, string] = .Err("bad")
	var success Result[int?, string] = .Ok(nil)
	var implicit int? = 2
	var typedNil (*int)? = pointer
	var nested (int?)? = present
	var call int? = identityOption(.Some(3))
	var branch int? = if flag { .Some(4) } else { .None }
	var match int? = switch flag {
	case true => .Some(5)
	case false => .None
	}
	var qualified Option[int] = Option[int].Some(6)           // want "should omit type Option\\[int\\]"
	var ordinary int = 7                                      // want "should omit type int"
	var factory func() int? = func() int? { return .Some(8) } // want "should omit type func\\(\\) int\\?"
	_, _, _, _, _, _, _, _, _, _, _, _, _, _ = absent, explicit, present, failure, success, implicit, typedNil, nested, call, branch, match, qualified, ordinary, factory
}
