package gonfixture
var f func([]int) = func(xs []int){
 for _,x:=range xs {
  if x>0{println(x)}
  return // want "the surrounding loop is unconditionally terminated"
 }
}
var g func([]int) = (xs) => {
 for _,x:=range xs {
  if x>0{println(x)}
  return // want "the surrounding loop is unconditionally terminated"
 }
}
