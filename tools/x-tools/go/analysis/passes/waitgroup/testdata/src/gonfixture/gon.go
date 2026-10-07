package gon
import "sync"
type Task func()
func f(){
 var wg sync.WaitGroup
 go func(){
  wg.Add(1) // want "WaitGroup.Add called from inside new goroutine"
  wg.Done()
 }()
 go Task(() => {
  wg.Add(1) // want "WaitGroup.Add called from inside new goroutine"
  wg.Done()
 })()
 go Task(() => wg.Add(1))() // want "WaitGroup.Add called from inside new goroutine"
 // Passing a callback to another function does not prove immediate invocation.
 go take(() => {wg.Add(1);wg.Done()})
}
func take(Task){}
