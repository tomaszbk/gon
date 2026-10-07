package gonfixture
var f func(int) = func(x int){ // want "argument x is overwritten before first use"
 x=2
 println(x)
}
var g func(int) = (x) => { // want "argument x is overwritten before first use"
 x=2
 println(x)
}
