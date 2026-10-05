package gonfeatures

import "sync"

type holder struct{ Mu sync.Mutex }

func legacy(p *holder) any {
	if p != nil {
		return p.Mu // want "return copies lock value"
	}
	return sync.Mutex{}
}

func modern(p *holder) any {
	return p?.Mu ?? sync.Mutex{} // want "return copies lock value"
}

var old = func(mu sync.Mutex) {}                 // want "func passes lock by value"
var modernCallback func(sync.Mutex) = (mu) => {} // want "lambda passes lock by value"
