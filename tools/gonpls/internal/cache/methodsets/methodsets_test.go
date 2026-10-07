package methodsets_test

import (
	"go/types"
	"slices"
	"testing"

	"golang.org/x/tools/gopls/internal/cache/methodsets"
	"golang.org/x/tools/internal/testfiles"
	"golang.org/x/tools/txtar"
)

// TestGenericEnumMethods indexes method sets whose signatures mention
// generic enum values, and searches them
// across packages. Interface satisfaction must distinguish instantiations
// of Outcome and survive encoding of the index.
func TestGenericEnumMethods(t *testing.T) {
	const src = `
-- go.mod --
module example.com

go 1.27

-- svc/svc.go --
package svc

type Course struct{}
type Outcome[T, E any] enum { default Ok(T); Err(E) }

type Store interface {
	Load(id int) Outcome[Course, error]
	Save(c Course) Outcome[struct{}, error]
}

// IntLoader has the method name of Store but another instantiation of Outcome.
type IntLoader interface{ Load(id int) Outcome[int, error] }

type Mem struct{}

func (Mem) Load(id int) Outcome[Course, error] { return Outcome[Course, error].Ok(Course{}) }
func (Mem) Save(c Course) Outcome[struct{}, error] { return Outcome[struct{}, error].Ok(struct{}{}) }

type Counter struct{}

func (Counter) Load(id int) Outcome[int, error] { return Outcome[int, error].Ok(id) }

type Box[T any] struct{ v T }

func (b Box[T]) Get() Outcome[T, error] { return Outcome[T, error].Ok(b.v) }

type IntGetter interface{ Get() Outcome[int, error] }
type StringGetter interface{ Get() Outcome[string, error] }

-- app/app.go --
package app

import "example.com/svc"

type Remote struct{}

func (Remote) Load(id int) svc.Outcome[svc.Course, error] { return svc.Outcome[svc.Course, error].Ok(svc.Course{}) }
func (Remote) Save(c svc.Course) svc.Outcome[struct{}, error] { return svc.Outcome[struct{}, error].Ok(struct{}{}) }

type Partial struct{}

func (Partial) Load(id int) svc.Outcome[svc.Course, error] { return svc.Outcome[svc.Course, error].Ok(svc.Course{}) }
`
	pkgs := testfiles.LoadPackages(t, txtar.Parse([]byte(src)), "./svc", "./app")
	var svc *types.Package
	indexes := make(map[string]*methodsets.Index)
	for _, pkg := range pkgs {
		if pkg.PkgPath == "example.com/svc" {
			svc = pkg.Types
		}
		index := methodsets.NewIndex(pkg.Fset, pkg.Types)
		indexes[pkg.PkgPath] = methodsets.Decode(index.PkgPath, index.Encode())
	}

	// search returns the types in each package that implement
	// the named interface of package svc.
	search := func(iface string) map[string][]string {
		t.Helper()
		key, ok := methodsets.KeyOf(svc.Scope().Lookup(iface).Type())
		if !ok {
			t.Fatalf("%s has no methods", iface)
		}
		got := make(map[string][]string)
		for path, index := range indexes {
			for _, res := range index.Search(key, methodsets.Subtype, nil) {
				got[path] = append(got[path], res.TypeName)
			}
			slices.Sort(got[path])
		}
		return got
	}
	check := func(iface string, wantSvc, wantApp []string) {
		t.Helper()
		got := search(iface)
		if !slices.Equal(got["example.com/svc"], wantSvc) || !slices.Equal(got["example.com/app"], wantApp) {
			t.Errorf("implementations of %s: got svc=%v app=%v, want svc=%v app=%v",
				iface, got["example.com/svc"], got["example.com/app"], wantSvc, wantApp)
		}
	}
	check("Store", []string{"Mem", "Store"}, []string{"Remote"})
	check("IntLoader", []string{"Counter", "IntLoader"}, nil)

	// A method of a generic receiver that returns Outcome[T, error] has a
	// fingerprint that must be unified with the interface's.
	check("IntGetter", []string{"Box", "IntGetter"}, nil)
	check("StringGetter", []string{"Box", "StringGetter"}, nil)
}
