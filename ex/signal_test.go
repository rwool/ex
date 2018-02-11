package ex_test

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/rwool/ex/ex"
	"github.com/rwool/ex/test/helpers/testlogger"

	"github.com/rwool/ex/log"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestSendSignal(t *testing.T) {
	defer goroutinechecker.New(t, false)()
	goroutinechecker.SignalsUsed()

	logger, logBuf := testlogger.NewTestLogger(t, log.Warn)

	sigC := make(chan string)
	err := ex.BeginSignalHandling(logger, func(s ex.Signal) {
		sigC <- s.String()
	})
	require.NoError(t, err)
	defer ex.StopSignalHandling()

	s := ex.SIGUSR1

	ex.SendSignal(s)

	var out string
	select {
	case out = <-sigC:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for signal handler to be called")
	}

	assert.Equal(t, s.String(), out, "unexpected signal")
	assert.Empty(t, string(logBuf.BytesCopy()), "unexpected log output")
}

func TestSendOSSignal(t *testing.T) {
	defer goroutinechecker.New(t, false)()
	goroutinechecker.SignalsUsed()

	logger, logBuf := testlogger.NewTestLogger(t, log.Warn)

	sig := syscall.SIGHUP
	sigC := make(chan ex.Signal)
	err := ex.BeginSignalHandling(logger, nil, ex.SIGHUP, func(s ex.Signal) {
		sigC <- s
	})
	require.NoError(t, err, "unexpected error from starting signal handling")
	defer ex.StopSignalHandling()

	err = syscall.Kill(os.Getpid(), sig)
	require.NoError(t, err, "unexpected error from sending signal")

	select {
	case s := <-sigC:
		assert.Equal(t, ex.SIGHUP, s, "mismatched signal")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for signal handler to be called")
	}

	assert.Empty(t, string(logBuf.BytesCopy()), "unexpected log output")
}

func TestSignalPanic(t *testing.T) {
	defer goroutinechecker.New(t, false)()
	goroutinechecker.SignalsUsed()

	// Logging expected, so don't use testlogger here.
	logBuf := &testlogger.Buffer{} // Thread-safe buffer.
	logger := log.NewLogger(logBuf, log.Error)

	err := ex.BeginSignalHandling(logger, func(ex.Signal) {
		panic("test panic")
	})
	require.NoError(t, err, "unexpected error starting signal handling")
	defer ex.StopSignalHandling()

	// OS signal.
	err = syscall.Kill(os.Getpid(), syscall.SIGQUIT)
	require.NoError(t, err, "unexpected error sending signal")
	time.Sleep(20 * time.Millisecond)
	assert.Contains(t, logBuf.String(), "panic from calling handler for OS signal")

	// Virtual signal.
	ex.SendSignal(ex.SIGKILL)
	time.Sleep(20 * time.Millisecond)
	assert.Contains(t, logBuf.String(), "panic from calling handler for non-OS signal")
}

func TestSignalBadBeginHandlingArguments(t *testing.T) {
	defer goroutinechecker.New(t, false)()
	goroutinechecker.SignalsUsed()

	// Nil logger.
	assert.Panics(t, func() {
		ex.BeginSignalHandling(nil, nil)
	})

	logger, _ := testlogger.NewTestLogger(t, log.Warn)

	// Incorrect number of handler arguments.
	assert.Panics(t, func() {
		ex.BeginSignalHandling(logger, nil, ex.SIGQUIT)
	})

	// Incorrect type for signal argument (sycall.Signal).
	assert.Panics(t, func() {
		ex.BeginSignalHandling(logger, nil, syscall.SIGHUP, func(ex.Signal) {})
	})

	// Incorrect type for signal argument (other).
	assert.Panics(t, func() {
		ex.BeginSignalHandling(logger, nil, "SIGHUP", func(ex.Signal) {})
	})

	// Incorrect type for function argument.
	assert.Panics(t, func() {
		ex.BeginSignalHandling(logger, nil, ex.SIGHUP, func() {})
	})
}

func TestSignalBeginHandlingTwiceError(t *testing.T) {
	defer goroutinechecker.New(t, false)()
	goroutinechecker.SignalsUsed()

	logger, _ := testlogger.NewTestLogger(t, log.Warn)

	err := ex.BeginSignalHandling(logger, nil)
	require.NoError(t, err, "unexpected error starting signal handling")
	defer ex.StopSignalHandling()

	err = ex.BeginSignalHandling(logger, nil)
	require.EqualError(t, err, ex.ErrHandlersAlreadySet.Error(),
		"unexpected error from starting signal handling")
}
