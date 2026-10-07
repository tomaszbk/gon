package golang

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/parsego"
)

// enumVariantDecl recognizes constructor declarations, which are scoped to an
// enum rather than represented as package-level function or variable specs.
func enumVariantDecl(pgf *parsego.File, pos token.Pos) (*ast.EnumVariant, *ast.TypeSpec) {
	cur, ok := pgf.Cursor().FindByPos(pos, pos)
	if !ok {
		return nil, nil
	}
	var variant *ast.EnumVariant
	for p := cur; p.Node() != nil; p = p.Parent() {
		switch n := p.Node().(type) {
		case *ast.EnumVariant:
			if pos == n.Name.Pos() {
				variant = n
			}
		case *ast.TypeSpec:
			if variant != nil {
				return variant, n
			}
			return nil, nil
		}
	}
	return nil, nil
}

// enumObjectOwner recovers the declaration owner of a descriptor object.
// Descriptor positions are preserved through generic substitutions and export.
func enumObjectOwner(obj types.Object) *types.TypeName {
	if obj.Pkg() == nil || obj.Parent() != nil {
		return nil
	}
	typ := obj.Type()
	if sig, ok := typ.(*types.Signature); ok && sig.Results().Len() == 1 {
		typ = sig.Results().At(0).Type()
	}
	if named, ok := types.Unalias(typ).(*types.Named); ok && enumContainsObject(named.Origin().Obj(), obj) {
		return named.Origin().Obj()
	}
	for _, name := range obj.Pkg().Scope().Names() {
		owner, ok := obj.Pkg().Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if enumContainsObject(owner, obj) {
			return owner
		}
	}
	return nil
}

func enumContainsObject(owner *types.TypeName, obj types.Object) bool {
	if enum := types.EnumOf(owner.Type()); enum != nil {
		for i := 0; i < enum.NumVariants(); i++ {
			variant := enum.Variant(i)
			if sameEnumObject(obj, variant.Object()) {
				return true
			}
			for j := 0; j < variant.NumFields(); j++ {
				if sameEnumObject(obj, variant.Field(j)) {
					return true
				}
			}
		}
	}
	return false
}

func enumObjectOwnerInPackage(pkg *cache.Package, obj types.Object) *types.TypeName {
	if obj.Parent() != nil || obj.Pkg() == nil {
		return nil
	}
	if owner := enumObjectOwner(obj); owner != nil {
		return owner
	}
	// Payload fields of a local enum have no lexical parent and their types
	// need not refer back to their enum. Find the enclosing declaration from
	// the checked objects rather than interpreting its backing struct.
	for _, def := range pkg.TypesInfo().Defs {
		if owner, ok := def.(*types.TypeName); ok && enumContainsObject(owner, obj) {
			return owner
		}
	}
	return nil
}

func sameEnumObject(a, b types.Object) bool {
	return a == b || a.Pos().IsValid() && a.Pos() == b.Pos() && a.Name() == b.Name() && a.Pkg() == b.Pkg()
}

// stringEnumGeneratedOwner identifies declarations supplied by the compiler.
// Their source location is the owning type, and their spelling cannot be
// changed independently through a source refactor.
func stringEnumGeneratedOwner(obj types.Object) *types.TypeName {
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	sig := fn.Signature()
	var owner types.Type
	if sig.Recv() != nil {
		owner = sig.Recv().Type()
		if ptr, ok := owner.(*types.Pointer); ok {
			owner = ptr.Elem()
		}
	} else if fn.Name() == "Parse" && sig.Results().Len() == 1 {
		owner = sig.Results().At(0).Type()
	} else {
		return nil
	}
	named, ok := types.Unalias(owner).(*types.Named)
	if !ok {
		return nil
	}
	enum := types.EnumOf(named)
	if enum == nil || !enum.IsString() || fn.Pos() != named.Obj().Pos() {
		return nil
	}
	switch fn.Name() {
	case "Parse":
		if parser := enum.StringParser(); parser != nil && sameEnumObject(fn, parser) {
			return named.Obj()
		}
	case "String", "MarshalText", "UnmarshalText":
		if sig.Recv() != nil {
			return named.Obj()
		}
	}
	return nil
}

func (r *renamer) checkEnumObject(from types.Object, owner *types.TypeName) {
	if r.to == "_" {
		r.errorf(from.Pos(), "enum variants and payload fields require a non-blank name")
		return
	}
	if from.Exported() && !ast.IsExported(r.to) && r.pkg.Types() != from.Pkg() {
		if id := someUse(r.pkg.TypesInfo(), from); id != nil {
			r.checkExport(id, r.pkg.Types(), from)
		}
	}
	enum := types.EnumOf(owner.Type())
	if enum.IsString() && r.to == "Parse" {
		r.errorf(from.Pos(), "renaming enum member to Parse would collide with the automatic parser")
		return
	}
	for i := 0; i < enum.NumVariants(); i++ {
		variant := enum.Variant(i)
		if sameEnumObject(from, variant.Object()) {
			for j := 0; j < enum.NumVariants(); j++ {
				other := enum.Variant(j).Object()
				if j != i && other.Name() == r.to {
					r.errorf(from.Pos(), "renaming variant %q to %q would duplicate an alternative", from.Name(), r.to)
				}
			}
			if named, ok := types.Unalias(owner.Type()).(*types.Named); ok {
				for j := 0; j < named.NumMethods(); j++ {
					if named.Method(j).Name() == r.to {
						r.errorf(from.Pos(), "renaming variant %q to %q would collide with an enum method", from.Name(), r.to)
					}
				}
			}
			return
		}
		for j := 0; j < variant.NumFields(); j++ {
			if sameEnumObject(from, variant.Field(j)) {
				for k := 0; k < variant.NumFields(); k++ {
					if k != j && variant.Field(k).Name() == r.to {
						r.errorf(from.Pos(), "renaming payload field %q to %q would duplicate a field", from.Name(), r.to)
					}
				}
				return
			}
		}
	}
}
