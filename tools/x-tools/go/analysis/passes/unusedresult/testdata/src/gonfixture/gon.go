package gon
import "fmt"
type S struct{}
func (*S) String()string{return ""}
func f(p *S){
 p.String() // want "result of .*String call not used"
 p?.String() // want "result of .*String call not used"
 fmt.Sprintf("hello") // want "result of fmt.Sprintf call not used"
 fmt.Sprintf?("hello") // want "result of fmt.Sprintf call not used"
}
