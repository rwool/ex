// Package goroutinechecker implements checking for leftover goroutines from
// running tests.
package goroutinechecker

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Variables global due to goroutines being global.
var (
	signalsUsed   bool
	signalsUsedMu sync.Mutex
)

// GetStack returns a stack trace of all running goroutines.
func GetStack() string {
	buf := make([]byte, 1<<20)
	read := runtime.Stack(buf, true)
	return string(buf[:read])
}

// SignalsUsed is used to make it so that the goroutine check will not fail due
// to the extra goroutine created from Go's signal handler.
//
// Ostensibly there is no way of stopping Go's signal handler goroutine once
// it has started, so the number of expected goroutines will increase by one
// once the handler is started.
func SignalsUsed() {
	signalsUsedMu.Lock()
	signalsUsed = true
	signalsUsedMu.Unlock()
}

func isSignalsUsed() bool {
	signalsUsedMu.Lock()
	defer signalsUsedMu.Unlock()
	return signalsUsed
}

// New checks and returns a function that checks to see if no excessive
// goroutines have been created.
//
// This is used to ensure that tests do not interfere with each other due to
// possible excessive resource usage.
func New(t *testing.T, isSubTest bool) func() {
	t.Helper()

	var numGR int
	if isSubTest {
		numGR = 4
	} else {
		numGR = 3
	}

	// Check the number of goroutines.
	//
	// In case some goroutines are in the process of closing, wait a bit before
	// checking again.
	checkGR := func(s string) {
		if isSignalsUsed() {
			numGR++
		}

		if runtime.NumGoroutine() > numGR {
			time.Sleep(50 * time.Millisecond)
			msgFmt := "too many goroutines at test %s (have %d, but expected %d):\n%s"
			curGR := runtime.NumGoroutine()
			assert.True(t, runtime.NumGoroutine() <= numGR, msgFmt, s, curGR, numGR, GetStack())
		}
	}

	checkGR("start")

	return func() {
		checkGR("end")
	}
}
