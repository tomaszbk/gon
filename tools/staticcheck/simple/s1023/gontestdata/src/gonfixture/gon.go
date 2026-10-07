package gonfixture
var f func() = func(){println(1);return} // want "redundant return statement"
var g func() = () => {println(1);return} // want "redundant return statement"
var h func()int = () => {return 1}
