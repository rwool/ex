package goroutinechecker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sync"

	"github.com/rwool/ex/test/helpers/goroutinechecker"
	"github.com/stretchr/testify/require"
)

func TestGoroutineChecker(t *testing.T) {
	defer goroutinechecker.New(t, false)()
	t.Run("SubTest", func(t2 *testing.T) {
		defer goroutinechecker.New(t2, true)
	})
}

func TestGetStack(t *testing.T) {
	assert.True(t, len(goroutinechecker.GetStack()) > 0, "got empty stack trace")
}

func TestCheckNumGoroutines(t *testing.T) {
	// Check plus 1 goroutine.
	startGR := goroutinechecker.CurrentGR()

	stopC := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-stopC
	}()

	pass, _ := goroutinechecker.CheckNumGoroutines(startGR)
	require.False(t, pass, "unexpected goroutine count check pass")
	close(stopC)
	wg.Wait()

	// Check plus 0 goroutines.
	startGR = goroutinechecker.CurrentGR()
	pass, _ = goroutinechecker.CheckNumGoroutines(startGR)
	require.True(t, pass, "unexpected failure from goroutine count check")
}
