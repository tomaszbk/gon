package gon
import "fmt"
var legacy func(string,...any) = func(format string,args ...any){ fmt.Printf(format,args...) } // want legacy:"printfWrapper"
var modern func(string,...any) = (format,args) => { fmt.Printf(format,args...) } // want modern:"printfWrapper"
var expression func(string,...any)(int,error) = (format,args) => fmt.Printf(format,args...) // want expression:"printfWrapper"
func f(){
 legacy("%d","bad") // want "legacy format %d has arg .* of wrong type string"
 modern("%d","bad") // want "modern format %d has arg .* of wrong type string"
 expression("%d","bad") // want "expression format %d has arg .* of wrong type string"
 fmt.Printf?("%d","bad") // want "fmt.Printf format %d has arg .* of wrong type string"
}

func interpolation(n int, text string) {
 _ = $"n=${n:%d}, text=${text:%q}"
 _ = $"bad=${text:%d}" // want "fmt.Sprintf format %d has arg .* of wrong type string"
 _ = $"bad=${n:%s}" // want "fmt.Sprintf format %s has arg .* of wrong type int"
}
