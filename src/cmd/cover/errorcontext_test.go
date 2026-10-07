package main_test

import (
	"internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Error-context handlers return in the middle of a statement. Coverage must
// give the following statement a separate counter, even for a nil context.
func TestErrorContextCoverage(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveExec(t)
	if coverageBaseline() == "" {
		t.Fatal("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	const common = `package contextflow
import "errors"
var Steps int
func work(fail bool) (int,error) { if fail { return 99,errors.New("boom") }; return 7,nil }
func context(err error) error { Steps++; return err }
`
	const modern = `package contextflow
func Run(fail bool) (int,error) {
 Steps++ // BEFORE
 n := work(fail) or err => context(err) // CALL
 Steps++ // AFTER
 return n,nil // RETURN
}
`
	const legacy = `package contextflow
func Run(fail bool) (int,error) {
 Steps++ // BEFORE
 n,err := work(fail) // CALL
 if err != nil { return 0,context(err) }
 Steps++ // AFTER
 return n,nil // RETURN
}
`
	const tests = `package contextflow
import "testing"
func TestSuccess(t *testing.T) { n,err:=Run(false);if n!=7||err!=nil||Steps!=2{t.Fatal(n,err,Steps)} }
func TestFailure(t *testing.T) { n,err:=Run(true);if n!=0||err==nil||Steps!=2{t.Fatal(n,err,Steps)} }
`
	for _, tc := range errorFlowToolchains(t) {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := legacy
			if tc.variant == "modern" { source = modern }
			for name, content := range map[string]string{"go.mod":"module contextflow\n\ngo 1.27\n", "common.go":common, "flow.go":source, "flow_test.go":tests} {
				if err:=os.WriteFile(filepath.Join(dir,name),[]byte(content),0600);err!=nil{t.Fatal(err)}
			}
			for _, path := range []struct{ name string; count int }{{"Success",1},{"Failure",0}} {
				profile := filepath.Join(dir, path.name+".out")
				cmd := tc.command(t,dir,"test","-vet=off","-count=1","-run=^Test"+path.name+"$","-covermode=count","-coverprofile="+profile,".")
				if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("%s: %v\n%s",path.name,err,out)}
				blocks:=readProfileBlocks(t,profile,"flow.go","count")
				for i,line:=range strings.Split(source,"\n") {
					want:=-1
					if strings.Contains(line,"// BEFORE")||strings.Contains(line,"// CALL"){want=1}
					if strings.Contains(line,"// AFTER")||strings.Contains(line,"// RETURN"){want=path.count}
					if want<0{continue}
					found:=false
					for _,block:=range blocks{if block.startLine<=i+1&&i+1<=block.endLine{found=true;if block.count!=want{t.Errorf("%s line %d: count=%d, want=%d",path.name,i+1,block.count,want)}}}
					if !found{t.Errorf("%s line %d has no counter",path.name,i+1)}
				}
			}
		})
	}
}
