package signal_test

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/rwool/ex/test/helpers/testlogger"

	"github.com/rwool/ex/log"

	"github.com/stretchr/testify/require"

	"github.com/rwool/ex/ex/internal/signal"
	"github.com/stretchr/testify/assert"
)

func TestSendSignal(t *testing.T) {
	defer goroutinechecker.New(t)()

	logger, logBuf := testlogger.NewTestLogger(t, log.Warn)

	sigC := make(chan string)
	err := signal.BeginSignalHandling(logger, func(s signal.Signal) {
		sigC <- s.String()
	})
	require.NoError(t, err)
	defer signal.StopSignalHandling()

	s := signal.SIGUSR1

	signal.SendSignal(s)

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
	defer goroutinechecker.New(t)()

	logger, logBuf := testlogger.NewTestLogger(t, log.Warn)

	sig := syscall.SIGHUP
	sigC := make(chan signal.Signal)
	err := signal.BeginSignalHandling(logger, nil, signal.SIGHUP, func(s signal.Signal) {
		sigC <- s
	})
	require.NoError(t, err, "unexpected error from starting signal handling")
	defer signal.StopSignalHandling()

	err = syscall.Kill(os.Getpid(), sig)
	require.NoError(t, err, "unexpected error from sending signal")

	select {
	case s := <-sigC:
		assert.Equal(t, signal.SIGHUP, s, "mismatched signal")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for signal handler to be called")
	}

	assert.Empty(t, string(logBuf.BytesCopy()), "unexpected log output")
}

func TestSignalPanic(t *testing.T) {
	defer goroutinechecker.New(t)()

	// Logging expected, so don't use testlogger here.
	logBuf := &testlogger.Buffer{} // Thread-safe buffer.
	logger := log.NewLogger(logBuf, log.Error)

	err := signal.BeginSignalHandling(logger, func(signal.Signal) {
		panic("test panic")
	})
	require.NoError(t, err, "unexpected error starting signal handling")
	defer signal.StopSignalHandling()

	// OS signal.
	err = syscall.Kill(os.Getpid(), syscall.SIGQUIT)
	require.NoError(t, err, "unexpected error sending signal")
	time.Sleep(20 * time.Millisecond)
	assert.Contains(t, logBuf.String(), "panic from calling handler for OS signal")

	// Virtual signal.
	signal.SendSignal(signal.SIGKILL)
	time.Sleep(20 * time.Millisecond)
	assert.Contains(t, logBuf.String(), "panic from calling handler for non-OS signal")
}

func TestSignalBadBeginHandlingArguments(t *testing.T) {
	defer goroutinechecker.New(t)()

	// Nil logger.
	assert.Panics(t, func() {
		signal.BeginSignalHandling(nil, nil)
	})

	logger, _ := testlogger.NewTestLogger(t, log.Warn)

	// Incorrect number of handler arguments.
	assert.Panics(t, func() {
		signal.BeginSignalHandling(logger, nil, signal.SIGQUIT)
	})

	// Incorrect type for signal argument (sycall.Signal).
	assert.Panics(t, func() {
		signal.BeginSignalHandling(logger, nil, syscall.SIGHUP, func(signal.Signal) {})
	})

	// Incorrect type for signal argument (other).
	assert.Panics(t, func() {
		signal.BeginSignalHandling(logger, nil, "SIGHUP", func(signal.Signal) {})
	})

	// Incorrect type for function argument.
	assert.Panics(t, func() {
		signal.BeginSignalHandling(logger, nil, signal.SIGHUP, func() {})
	})
}

func TestSignalBeginHandlingTwiceError(t *testing.T) {
	defer goroutinechecker.New(t)()

	logger, _ := testlogger.NewTestLogger(t, log.Warn)

	err := signal.BeginSignalHandling(logger, nil)
	require.NoError(t, err, "unexpected error starting signal handling")
	defer signal.StopSignalHandling()

	err = signal.BeginSignalHandling(logger, nil)
	require.EqualError(t, err, signal.ErrHandlersAlreadySet.Error(),
		"unexpected error from starting signal handling")
}
