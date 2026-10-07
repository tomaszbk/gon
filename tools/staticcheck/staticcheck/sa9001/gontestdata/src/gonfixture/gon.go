package gonfixture
func use(func()){}
func f(ch chan int){for range ch {
 defer println(1) // want "defers in this range loop won't run unless the channel gets closed"
 use(func(){return})
 use(() => {return})
}}
func nested(ch chan int){for range ch {
 use(func(){defer println(1)})
 use(() => {defer println(1)})
}}
