// Package goroutinechecker implements checking for leftover goroutines from
// running tests.
package goroutinechecker

import (
	"runtime"
	"testing"
	"time"

	"bufio"
	"bytes"

	"github.com/stretchr/testify/assert"
)

func getStackBytes() []byte {
	buf := make([]byte, 1<<20)
	read := runtime.Stack(buf, true)
	return buf[:read]
}

// GetStack returns a stack trace of all running goroutines.
func GetStack() string {
	return string(getStackBytes())
}

// SignalsUsed is used to make it so that the goroutine check will not fail due
// to the extra goroutine created from Go's signal handler.
//
// Ostensibly there is no way of stopping Go's signal handler goroutine once
// it has started, so the number of expected goroutines will increase by one
// once the handler is started.
//
// Deprecated: Has no effect now.
func SignalsUsed() {}

// getIgnorableCount gets the number of stacks that can be excluded from the
// count of "effective" goroutines.
func getIgnorableCount(stack []byte) int {
	var count int
	s := bufio.NewScanner(bytes.NewReader(stack))
	for s.Scan() {
		has := func(str string) bool { return bytes.Contains(s.Bytes(), []byte(str)) }
		if has("testing.(*T).Parallel") ||
			has("signal.signal_recv") ||
			has("runtime.gopark") {
			count++
		}
	}
	if err := s.Err(); err != nil {
		panic(err)
	}
	return count
}

// CurrentGR gets the current "effective" number of goroutines.
//
// This is gets the number of goroutines and compensates for stacks from
// parallel test runs and the signal handler stack.
func CurrentGR() int {
	return runtime.NumGoroutine() - getIgnorableCount(getStackBytes())
}

// CheckNumGoroutines checks if the "effective" number of goroutines is less
// than or equal to the provided previous goroutine count.
func CheckNumGoroutines(prevGR int) (passed bool, effectiveGR int) {
	curGR := CurrentGR()
	if curGR > prevGR {
		// Wait for goroutines to finish.
		time.Sleep(50 * time.Millisecond)
		curGR = CurrentGR()
		return curGR <= prevGR, curGR
	}
	return true, curGR
}

// New checks and returns a function that checks to see if no excessive
// goroutines have been created.
//
// This is used to ensure that tests do not interfere with each other due to
// possible excessive resource usage.
//
// This function should not be used in tests that also call the T.Parallel
// method.
func New(t *testing.T, _ bool) func() {
	t.Helper()

	startingGR := runtime.NumGoroutine()

	return func() {
		passed, gr := CheckNumGoroutines(startingGR)
		if !passed {
			msgFmt := "too many goroutines at test end (have %d, but expected %d):\n%s"
			assert.FailNow(t, "too many goroutines", msgFmt, gr, startingGR, GetStack())
		}
	}
}
