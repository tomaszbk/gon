package cfg

import "go/ast"

// match keeps pattern tests, guards and arm bodies on separate paths. Pattern
// syntax does not evaluate arbitrary expressions: heads select variants and
// identifiers bind payload copies. This type-free graph conservatively treats
// every non-default pattern as a test; SSA has the exact typed pattern tree.
func (b *builder) match(e *ast.MatchExpr, stmt ast.Stmt, label *lblock) {
	b.add(e.Tag)
	done := b.newBlock(KindMatchDone, stmt)
	saved := b.targets
	if stmt != nil {
		b.targets = &targets{tail: saved, _break: done}
		if label != nil {
			label._break = done
		}
	}
	for i, arm := range e.Arms {
		body := b.newBlock(KindMatchArm, stmt)
		next := b.newBlock(KindMatchTest, stmt)
		// Exhaustiveness is mandatory in well-typed Gon matches, so the
		// final unguarded arm covers all values surviving earlier tests.
		if len(arm.Patterns) == 0 || i == len(e.Arms)-1 && arm.Guard == nil {
			b.jump(body)
		} else {
			for j, pattern := range arm.Patterns {
				failure := next
				if j < len(arm.Patterns)-1 {
					failure = b.newBlock(KindMatchTest, stmt)
				}
				// Pattern heads and bindings are structural tests, never
				// arbitrary evaluated expressions. Success skips the other
				// alternatives and reaches the arm's single guard.
				b.current.Nodes = append(b.current.Nodes, pattern)
				b.ifelse(body, failure)
				b.current = failure
			}
		}
		b.current = body
		if arm.Guard != nil {
			guarded := b.newBlock(KindMatchArm, stmt)
			b.add(arm.Guard)
			b.ifelse(guarded, next)
			b.current = guarded
		}
		if stmt != nil {
			b.stmt(arm.Body)
		} else {
			b.add(arm.Value)
		}
		b.jump(done)
		b.current = next
	}
	b.jump(done)
	b.targets = saved
	b.current = done
}
