package gon
import "testing"
type Task func()
func TestGon(t *testing.T){
 go func(){t.Fatal("legacy")}() // want "call to .*Fatal from a non-test goroutine"
 go Task(() => t.Fatal("modern"))() // want "call to .*Fatal from a non-test goroutine"
 go Task(() => {t.Fatal("modern block")})() // want "call to .*Fatal from a non-test goroutine"
 t.Run("ok", (child) => { child.Fatal("in subtest goroutine") })
 t.Run("bad", (child) => { t.Fatal("outer") }) // want "call to .*Fatal on t defined outside of the subtest"
 var legacy func() = func(){t.Fatal("local")}
 var modern func() = () => {t.Fatal("local")}
 go legacy() // want "call to .*Fatal from a non-test goroutine"
 go modern() // want "call to .*Fatal from a non-test goroutine"
}
