package gonfixture
func use(func()){}
func f(){for {
 defer println(1) // want "defers in this infinite loop will never run"
 use(func(){return})
 use(() => {return})
}}
func nested(){for {
 use(func(){defer println(1)})
 use(() => {defer println(1)})
}}
