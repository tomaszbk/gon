package gon
import "time"
func sink(time.Duration){}
func callback(func()time.Duration){}
func f(t time.Time){
 defer sink(time.Since(t)) // want "call to time.Since is not deferred"
 defer callback(func()time.Duration{return time.Since(t)})
 defer callback(() => time.Since(t))
 defer callback(() => {return time.Since(t)})
}
