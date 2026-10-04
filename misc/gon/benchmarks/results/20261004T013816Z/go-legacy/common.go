// The runner copies this file unchanged into all three benchmark modules.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
)

const batchSize = 64

var (
	errNegative = errors.New("negative input")
	errLimit    = errors.New("amount exceeds limit")
	defaultRule = policy{Factor: 3}
	presentRule = policy{Factor: 2}
	zeroRule    = policy{}
)

type policy struct{ Factor int }

// Effects make skipped work observable, including argument and location
// evaluation. Both implementations use exactly these helpers.
type effects struct {
	Reads, Validations, Defaults, Arguments, Locations, TrueBranches, FalseBranches int64
	Constructions, Copies, MatchGuards, MatchBodies                                 int64
	OptionReads, OptionTransforms, OptionDefaults                                   int64
	ResultReads, ResultTransforms, ResultHandlers, ResultDefaults, NilFailures      int64
	Callees, NamedArguments, ArgumentTrace                                          int64
}

type input struct {
	Text     string
	Rule     *policy
	Callback func(int) int
	Active   bool
}

func readAmount(seed int, e *effects) (int, error) {
	e.Reads++
	if seed < 0 {
		return 99, errNegative // Deliberate partial result, discarded by both variants.
	}
	return seed*3 + 1, nil
}

func validateAmount(value int, e *effects) (int, error) {
	e.Validations++
	if value > 1000 {
		return value, errLimit
	}
	return value + 7, nil
}

func parseAmount(text string, e *effects) (int, error) {
	e.Reads++
	return strconv.Atoi(text)
}

func trueValue(value int, e *effects) int {
	e.TrueBranches++
	return value*3 + 1
}

func falseValue(value int, e *effects) int {
	e.FalseBranches++
	return value + 5
}

func defaultNumber(e *effects) int {
	e.Defaults++
	return 7
}

func defaultPolicy(e *effects) *policy {
	e.Defaults++
	return &defaultRule
}

func argument(value int, e *effects) int {
	e.Arguments++
	return value + 1
}

func location(index int, e *effects) int {
	e.Locations++
	return index
}

func double(value int) int { return value * 2 }
func zero(int) int         { return 0 }

func sumMapped(values []int, transform func(int) int) int {
	total := 0
	for _, value := range values {
		total += transform(value)
	}
	return total
}

var values = func() [batchSize]int {
	var result [batchSize]int
	for i := range result {
		result[i] = i - 16
	}
	return result
}()

func makeInputs(mode string) [batchSize]input {
	var result [batchSize]input
	for i := range result {
		item := input{Text: strconv.Itoa(i%24 + 1), Active: i%2 == 0}
		if i%3 == 0 {
			item.Rule = &presentRule
		}
		if i%4 == 0 {
			item.Callback = double
		}
		if mode == "failure" || mode == "mixed" && i%4 == 0 {
			if i%2 == 0 {
				item.Text = "invalid"
			} else {
				item.Text = "5000"
			}
		}
		// Mixed inputs include errors after parsing as well as parse failures.
		if mode == "mixed" && i%4 == 1 {
			item.Text = "5000"
		}
		result[i] = item
	}
	return result
}

var (
	successInputs = makeInputs("success")
	failureInputs = makeInputs("failure")
	mixedInputs   = makeInputs("mixed")
)

func propagationBatch(failure bool, e *effects) (int, int) {
	total, failures := 0, 0
	for i := 0; i < batchSize; i++ {
		seed := i
		if failure {
			seed = -1
			if i%2 != 0 {
				seed = 400
			}
		}
		value, err := propagate(seed, e)
		total += value
		if err != nil {
			failures++
		}
	}
	return total, failures
}

func handlerBatch(e *effects) (int, int) {
	total, failures := 0, 0
	for i := 0; i < batchSize; i++ {
		seed := i
		if i%4 == 0 {
			seed = -1
		}
		value, err := handle(seed, e)
		total += value
		if err != nil {
			failures++
		}
	}
	return total, failures
}

func conditionalBatch(e *effects) int {
	total := 0
	for i, value := range values {
		total += conditional(i%2 == 0, value, e)
	}
	return total
}

func fieldBatch(present bool, e *effects) int {
	total := 0
	for i := 0; i < batchSize; i++ {
		var rule *policy
		if present {
			rule = &presentRule
			if i%2 == 0 {
				rule = &zeroRule
			}
		}
		total += safeField(rule, e)
	}
	return total
}

func callBatch(e *effects) int {
	total := 0
	for i, value := range values {
		var callback func(int) int
		if i%3 == 0 {
			callback = double
		} else if i%3 == 1 {
			callback = zero
		}
		total += safeCall(callback, value, e)
	}
	return total
}

func assignmentBatch(e *effects) int {
	var slots [batchSize]*policy
	for i := range slots {
		if i%2 == 0 {
			slots[i] = &presentRule
		}
	}
	total := 0
	for i := range slots {
		total += coalesceAssign(slots[:], i, e)
	}
	return total
}

func pipelineBatch(inputs *[batchSize]input, e *effects) (int, int) {
	total, failures := 0, 0
	for _, item := range inputs {
		value, err := pipeline(item, e)
		total += value
		if err != nil {
			failures++
		}
	}
	return total, failures
}

var eventSource = func() [batchSize]benchEvent {
	var result [batchSize]benchEvent
	for i := range result {
		result[i] = constructEvent(i, nil)
	}
	return result
}()

var eventStorage [batchSize]benchEvent

func enumConstructBatch(e *effects) int {
	total := 0
	for i := range eventStorage {
		eventStorage[i] = constructEvent(i, e)
		total += i
	}
	return total
}

func enumCopyBatch(e *effects) int {
	total := 0
	for i, value := range eventSource {
		eventStorage[i] = value
		e.Copies++
		total += i
	}
	return total
}

func matchGuard(value int, e *effects) bool {
	e.MatchGuards++
	return value%2 == 0
}

func matchBody(value int, e *effects) int {
	e.MatchBodies++
	return value
}

func matchingBatch(guarded bool, e *effects) int {
	total := 0
	for _, value := range eventSource {
		total += matchEvent(value, guarded, e)
	}
	return total
}

func optionDefault(e *effects) int { e.OptionDefaults++; return 7 }
func resultDefault(e *effects) int { e.ResultDefaults++; return 11 }

func optionBatch(present bool, e *effects) int {
	total := 0
	for _, value := range values {
		total += optionScenario(value, present, e)
	}
	return total
}

func resultBatch(failed bool, e *effects) (int, int) {
	total, failures := 0, 0
	for i, value := range values {
		result, failure, _ := resultScenario(value, failed, i%2 == 0, e)
		total += result
		if failure {
			failures++
		}
	}
	return total, failures
}

func resultHandlerBatch(e *effects) (int, int) {
	total, failures := 0, 0
	for i, value := range values {
		result, failure := resultHandle(value, i%2 == 0, i%4 == 0, e)
		total += result
		if failure {
			failures++
		}
	}
	return total, failures
}

func namedCallee(e *effects) func(left, right int) int {
	e.Callees++
	e.ArgumentTrace = 1
	return namedTarget
}

func namedArgument(role, value int, e *effects) int {
	e.NamedArguments++
	// Bound the rolling trace even in lengthy benchmark runs.
	e.ArgumentTrace = (e.ArgumentTrace*4 + int64(role)) & 255
	return value
}

func namedTarget(left, right int) int { return left*3 + right }
func variadicTarget(prefix int, values ...int) int {
	total := prefix
	for _, value := range values {
		total += value
	}
	return total
}

func namedBatch(variadic bool, e *effects) int {
	total := 0
	for _, value := range values {
		if variadic {
			total += namedVariadic(value, e)
		} else {
			total += namedReordered(value, e)
		}
	}
	return total
}

type retentionMetrics struct {
	Count          int    `json:"count"`
	Checksum       int64  `json:"checksum"`
	SliceBytes     uint64 `json:"sliceBytes"`
	HeapDeltaBytes int64  `json:"heapDeltaBytes"`
	GCCycles       uint32 `json:"gcCycles"`
}

type representationMetrics struct {
	Sizes     map[string]uintptr `json:"sizesByShape"`
	Retention retentionMetrics   `json:"retention"`
}

func representation() representationMetrics {
	const count = 10000
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	root, sliceBytes := retainedPointers(count)
	runtime.GC()
	runtime.ReadMemStats(&after)
	checksum := retainedChecksum(root)
	runtime.KeepAlive(root)
	return representationMetrics{
		Sizes: representationSizes(),
		Retention: retentionMetrics{count, checksum, sliceBytes,
			int64(after.HeapAlloc) - int64(before.HeapAlloc), after.NumGC - before.NumGC},
	}
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--representation" {
		if err := json.NewEncoder(os.Stdout).Encode(representation()); err != nil {
			panic(err)
		}
		return
	}
	var e effects
	checksum, failures := propagationBatch(false, &e)
	value, count := propagationBatch(true, &e)
	checksum, failures = checksum+value, failures+count
	value, count = handlerBatch(&e)
	checksum, failures = checksum+value, failures+count
	checksum += conditionalBatch(&e) + capturedLambda(values[:], 5)
	checksum += fieldBatch(true, &e) + fieldBatch(false, &e)
	checksum += callBatch(&e) + assignmentBatch(&e)
	for _, inputs := range []*[batchSize]input{&successInputs, &failureInputs, &mixedInputs} {
		value, count = pipelineBatch(inputs, &e)
		checksum, failures = checksum+value, failures+count
	}
	checksum += enumConstructBatch(&e) + enumCopyBatch(&e)
	checksum += matchingBatch(false, &e) + matchingBatch(true, &e)
	checksum += optionBatch(true, &e) + optionBatch(false, &e) + optionAssignmentBatch(&e)
	for _, failed := range []bool{false, true} {
		value, count = resultBatch(failed, &e)
		checksum, failures = checksum+value, failures+count
	}
	value, count = resultHandlerBatch(&e)
	checksum, failures = checksum+value, failures+count
	checksum += namedBatch(false, &e) + namedBatch(true, &e)
	// The runner supplies buildNonce in nonce.go and changes it for each build.
	fmt.Printf("build=%d items_per_batch=%d checksum=%d failures=%d effects=%+v\n", buildNonce, batchSize, checksum, failures, e)
}
