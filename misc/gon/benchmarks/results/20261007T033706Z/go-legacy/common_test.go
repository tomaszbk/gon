// Tests and benchmark drivers are byte-identical in all three modules.
package main

import (
	"errors"
	"runtime"
	"strconv"
	"testing"
)

func TestPropagation(t *testing.T) {
	for _, tc := range []struct {
		seed, want int
		err        error
		effects    effects
	}{
		{0, 8, nil, effects{Reads: 1, Validations: 1}},
		{12, 44, nil, effects{Reads: 1, Validations: 1}},
		{333, 1007, nil, effects{Reads: 1, Validations: 1}},
		{-1, 0, errNegative, effects{Reads: 1}},
		{334, 0, errLimit, effects{Reads: 1, Validations: 1}},
	} {
		var e effects
		got, err := propagate(tc.seed, &e)
		if got != tc.want || err != tc.err || e != tc.effects {
			t.Errorf("seed %d: (%d, %v, %+v), want (%d, %v, %+v)", tc.seed, got, err, e, tc.want, tc.err, tc.effects)
		}
	}
}

func TestHandler(t *testing.T) {
	var e effects
	got, err := handle(-1, &e)
	if got != 0 || !errors.Is(err, errNegative) || err.Error() != "read amount: negative input" || e != (effects{Reads: 1}) {
		t.Fatalf("handler failure: %d, %v, %+v", got, err, e)
	}
	e = effects{}
	got, err = handle(12, &e)
	if got != 44 || err != nil || e != (effects{Reads: 1}) {
		t.Fatalf("handler success: %d, %v, %+v", got, err, e)
	}
}

func TestConditionalLaziness(t *testing.T) {
	var e effects
	if got := conditional(true, 4, &e); got != 13 || e != (effects{TrueBranches: 1}) {
		t.Fatalf("true branch: %d, %+v", got, e)
	}
	e = effects{}
	if got := conditional(false, 4, &e); got != 9 || e != (effects{FalseBranches: 1}) {
		t.Fatalf("false branch: %d, %+v", got, e)
	}
}

func TestLambdaCapture(t *testing.T) {
	if got := capturedLambda([]int{1, 2, 3}, 5); got != 33 {
		t.Fatalf("capture by reference: got %d, want 33", got)
	}
	if got := capturedLambda(nil, 5); got != 0 {
		t.Fatalf("empty input: got %d", got)
	}
	if got := capturedLambda([]int{-2, 0}, -2); got != -4 {
		t.Fatalf("zero updated offset: got %d", got)
	}
}

func TestSafeField(t *testing.T) {
	for _, tc := range []struct {
		rule     *policy
		want     int
		defaults int64
	}{{nil, 7, 1}, {&presentRule, 2, 0}, {&zeroRule, 0, 0}} {
		var e effects
		if got := safeField(tc.rule, &e); got != tc.want || e != (effects{Defaults: tc.defaults}) {
			t.Errorf("rule %v: %d, %+v", tc.rule, got, e)
		}
	}
}

func TestSafeCall(t *testing.T) {
	var e effects
	if got := safeCall(nil, 4, &e); got != 7 || e != (effects{Defaults: 1}) {
		t.Fatalf("nil call must skip argument: %d, %+v", got, e)
	}
	e = effects{}
	calls := 0
	callback := func(value int) int {
		calls++
		if e.Arguments != 1 || value != 5 {
			t.Fatalf("argument order: value %d, effects %+v", value, e)
		}
		return 0
	}
	if got := safeCall(callback, 4, &e); got != 0 || calls != 1 || e != (effects{Arguments: 1}) {
		t.Fatalf("present zero result must skip fallback: %d, %d, %+v", got, calls, e)
	}
	e = effects{}
	if got := safeCall(double, 4, &e); got != 10 || e != (effects{Arguments: 1}) {
		t.Fatalf("present result: %d, %+v", got, e)
	}
}

func TestCoalescingAssignment(t *testing.T) {
	slots := []*policy{nil, &zeroRule, &presentRule}
	var e effects
	for i, want := range []int{3, 0, 2} {
		if got := coalesceAssign(slots, i, &e); got != want {
			t.Fatalf("slot %d: got %d, want %d", i, got, want)
		}
	}
	if slots[0] != &defaultRule || slots[1] != &zeroRule || slots[2] != &presentRule || e != (effects{Defaults: 1, Locations: 3}) {
		t.Fatalf("single location evaluation and lazy defaults: %v, %+v", slots, e)
	}
	if got := coalesceAssign(slots, 0, &e); got != 3 || e != (effects{Defaults: 1, Locations: 4}) {
		t.Fatalf("second assignment: %d, %+v", got, e)
	}
}

func TestPipeline(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   input
		want    int
		err     error
		effects effects
	}{
		{"present", input{"10", &presentRule, double, true}, 133, nil, effects{Reads: 1, Validations: 1, Arguments: 1, TrueBranches: 1}},
		{"absent", input{"10", nil, nil, false}, 52, nil, effects{Reads: 1, Validations: 1, Defaults: 1, FalseBranches: 1}},
		{"zero factor", input{"0", &zeroRule, nil, true}, 7, nil, effects{Reads: 1, Validations: 1, TrueBranches: 1}},
		{"zero callback", input{"10", &presentRule, zero, true}, 7, nil, effects{Reads: 1, Validations: 1, Arguments: 1, TrueBranches: 1}},
		{"parse failure", input{"invalid", nil, double, true}, 0, strconv.ErrSyntax, effects{Reads: 1}},
		{"late failure", input{"5000", nil, nil, false}, 0, errLimit, effects{Reads: 1, Validations: 1, Defaults: 1, FalseBranches: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var e effects
			got, err := pipeline(tc.input, &e)
			if got != tc.want || !errors.Is(err, tc.err) || e != tc.effects {
				t.Fatalf("got (%d, %v, %+v), want (%d, %v, %+v)", got, err, e, tc.want, tc.err, tc.effects)
			}
		})
	}
}

func TestBatchCoverage(t *testing.T) {
	for _, tc := range []struct {
		inputs   *[batchSize]input
		failures int
	}{{&successInputs, 0}, {&failureInputs, batchSize}, {&mixedInputs, batchSize / 2}} {
		var e effects
		_, failures := pipelineBatch(tc.inputs, &e)
		if failures != tc.failures || e.Reads != batchSize {
			t.Fatalf("batch: failures %d, reads %d; want %d, %d", failures, e.Reads, tc.failures, batchSize)
		}
	}
}

func TestEnumConstructionAndCopies(t *testing.T) {
	var e effects
	if got := enumConstructBatch(&e); got != 2016 || e != (effects{Constructions: batchSize}) {
		t.Fatalf("construction: checksum %d, effects %+v", got, e)
	}
	for i, value := range eventStorage {
		var local effects
		want := 0
		switch i % 3 {
		case 1:
			want = i
		case 2:
			want = i*2 + 1
		}
		if got := matchEvent(value, false, &local); got != want || local.MatchBodies != 1 {
			t.Fatalf("active payload %d: %d, %+v; want %d", i, got, local, want)
		}
	}
	e = effects{}
	if got := enumCopyBatch(&e); got != 2016 || e != (effects{Copies: batchSize}) {
		t.Fatalf("copy: checksum %d, effects %+v", got, e)
	}
	original := eventStorage[4]
	copied := original
	copied = constructEvent(7, nil)
	if matchEvent(original, false, &e) != 4 || matchEvent(copied, false, &e) != 7 {
		t.Fatal("copy replacement changed original")
	}
}

func TestMatchingGuardsAndBodies(t *testing.T) {
	for _, guarded := range []bool{false, true} {
		for i := 0; i < 9; i++ {
			var e effects
			want, guards := 0, int64(0)
			switch i % 3 {
			case 1:
				want = i
				if guarded {
					guards = 1
					if i%2 == 0 {
						want *= 2
					}
				}
			case 2:
				want = i*2 + 1
			}
			got := matchEvent(constructEvent(i, nil), guarded, &e)
			if got != want || e != (effects{MatchGuards: guards, MatchBodies: 1}) {
				t.Fatalf("seed %d, guarded %t: %d, %+v; want %d, guards %d", i, guarded, got, e, want, guards)
			}
		}
	}
}

func TestOptionPresencePropagationAndDefaults(t *testing.T) {
	for _, present := range []bool{false, true} {
		for _, seed := range []int{-3, 0, 5} {
			var e effects
			want, expected := 7, effects{OptionReads: 1, OptionDefaults: 1}
			if present {
				want, expected = seed*2, effects{OptionReads: 1, OptionTransforms: 1}
			}
			if got := optionScenario(seed, present, &e); got != want || e != expected {
				t.Fatalf("Option seed %d, present %t: %d, %+v; want %d, %+v", seed, present, got, e, want, expected)
			}
		}
	}
	if !optionNilPresent() {
		t.Fatal("present nil payload became absence")
	}
	if got := optionNestedAbsent(); got != 7 {
		t.Fatalf("nested Option flattened: %d", got)
	}
}

func TestOptionAssignment(t *testing.T) {
	var e effects
	// Present even values sum to 992; 32 absent slots receive Some(7).
	if got := optionAssignmentBatch(&e); got != 1216 || e != (effects{Locations: batchSize, OptionDefaults: batchSize / 2}) {
		t.Fatalf("assignment must preserve present zero and evaluate each location once: %d, %+v", got, e)
	}
}

func TestNamedWrittenOrder(t *testing.T) {
	var e effects
	if got := namedReordered(5, &e); got != 21 || e != (effects{Callees: 1, NamedArguments: 2, ArgumentTrace: 30}) {
		t.Fatalf("callee first, right then left exactly once: %d, %+v", got, e)
	}
	e = effects{}
	if got := namedVariadic(5, &e); got != 18 || e != (effects{NamedArguments: 1, ArgumentTrace: 6}) {
		t.Fatalf("final slice variadic: %d, %+v", got, e)
	}
}

func TestRetainedPointerGC(t *testing.T) {
	root, bytes := retainedPointers(10000)
	runtime.GC()
	if got := retainedChecksum(root); got != 49995000 || bytes == 0 {
		t.Fatalf("retained payload after GC: checksum %d, slice bytes %d", got, bytes)
	}
	runtime.KeepAlive(root)
	sizes := representationSizes()
	if len(sizes) != 5 {
		t.Fatalf("representation shapes: %v", sizes)
	}
	for name, size := range sizes {
		if size == 0 {
			t.Errorf("missing representation size for %s", name)
		}
	}
}

var (
	resultSink  int
	failureSink int
	effectsSink effects
)

func reportBatch(b *testing.B) {
	b.ReportAllocs()
	b.ReportMetric(batchSize, "items/op")
}

func BenchmarkErrorPropagationSuccessBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink, failureSink = propagationBatch(false, &e)
	}
	effectsSink = e
}

func BenchmarkErrorPropagationFailureBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink, failureSink = propagationBatch(true, &e)
	}
	effectsSink = e
}

func BenchmarkErrorHandlerMixedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink, failureSink = handlerBatch(&e)
	}
	effectsSink = e
}

func BenchmarkConditionalMixedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = conditionalBatch(&e)
	}
	effectsSink = e
}

func BenchmarkLambdaCapturedBatch(b *testing.B) {
	reportBatch(b)
	for i := 0; i < b.N; i++ {
		resultSink = capturedLambda(values[:], i&7)
	}
}

func BenchmarkSafeFieldPresentBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = fieldBatch(true, &e)
	}
	effectsSink = e
}

func BenchmarkSafeFieldAbsentBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = fieldBatch(false, &e)
	}
	effectsSink = e
}

func BenchmarkSafeCallMixedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = callBatch(&e)
	}
	effectsSink = e
}

func BenchmarkCoalesceAssignMixedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = assignmentBatch(&e)
	}
	effectsSink = e
}

func BenchmarkPipelineSuccessBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink, failureSink = pipelineBatch(&successInputs, &e)
	}
	effectsSink = e
}

func BenchmarkPipelineFailureBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink, failureSink = pipelineBatch(&failureInputs, &e)
	}
	effectsSink = e
}

func BenchmarkPipelineMixedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink, failureSink = pipelineBatch(&mixedInputs, &e)
	}
	effectsSink = e
}

func BenchmarkEnumConstructionBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = enumConstructBatch(&e)
	}
	effectsSink = e
	runtime.KeepAlive(eventStorage)
}
func BenchmarkEnumCopyBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = enumCopyBatch(&e)
	}
	effectsSink = e
	runtime.KeepAlive(eventStorage)
}
func BenchmarkMatchingExhaustiveBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = matchingBatch(false, &e)
	}
	effectsSink = e
}
func BenchmarkMatchingGuardedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = matchingBatch(true, &e)
	}
	effectsSink = e
}
func BenchmarkOptionPresentBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = optionBatch(true, &e)
	}
	effectsSink = e
}
func BenchmarkOptionAbsentBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = optionBatch(false, &e)
	}
	effectsSink = e
}
func BenchmarkOptionAssignmentMixedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = optionAssignmentBatch(&e)
	}
	effectsSink = e
}
func BenchmarkNamedReorderedBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = namedBatch(false, &e)
	}
	effectsSink = e
}
func BenchmarkNamedVariadicBatch(b *testing.B) {
	reportBatch(b)
	var e effects
	for i := 0; i < b.N; i++ {
		resultSink = namedBatch(true, &e)
	}
	effectsSink = e
}

func TestFormattedBatch(t *testing.T) {
	// Ten one-digit and 54 two-digit counters, each with a colon and four digits.
	if got := formattedBatch(); got != 438 {
		t.Fatalf("formatted batch: %d", got)
	}
}
func BenchmarkInterpolationBatch(b *testing.B) {
	reportBatch(b)
	for b.Loop() {
		resultSink = formattedBatch()
	}
}
