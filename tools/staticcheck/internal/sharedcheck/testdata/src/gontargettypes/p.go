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
	var absent int? = nil
	var explicit (*int)? = (*int)(nil)
	var present int? = (int)(1)
	var implicit int? = 2
	var typedNil (*int)? = pointer
	var nested (int?)? = present
	var call int? = identityOption(3)
	var branch int? = if flag { 4 } else { nil }
	var match int? = switch flag {
	case true => 5
	case false => nil
	}
	var qualified int? = (int?)(6)
	var ordinary int = 7                               // want "should omit type int"
	var factory func() int? = func() int? { return 8 } // want "should omit type func\\(\\) int\\?"
	_, _, _, _, _, _, _, _, _, _, _, _ = absent, explicit, present, implicit, typedNil, nested, call, branch, match, qualified, ordinary, factory
}
