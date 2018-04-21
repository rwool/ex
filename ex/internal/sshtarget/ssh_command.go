package sshtarget

import (
	"context"
	errors2 "errors"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/rwool/ex/ex/internal/recorder"
	"github.com/rwool/ex/ex/internal/signal"
	"github.com/rwool/ex/log"
)

// ErrCancelledByTarget indicates command was cancelled indirectly by the
// target.
var ErrCancelledByTarget = errors2.New("cancelled by target")

// SSHSession is a single session within a SSH connection.
type SSHSession struct {
	ssh    *SSH
	conf   RunConfig
	rec    *recorder.Recorder
	logger log.Logger

	mu sync.Mutex

	errC chan error

	parentCtx, ctx context.Context

	finishFn func()
}

func newSSHSession(ctx context.Context, finishFn func()) *SSHSession {
	if ctx == nil {
		panic("nil context")
	}

	ss := &SSHSession{
		parentCtx: ctx,
	}

	if finishFn == nil {
		ss.finishFn = func() {}
	} else {
		ss.finishFn = finishFn
	}

	return ss
}

// LogEvent logs an event.
func (ss *SSHSession) LogEvent(eventType string, details interface{}) {
	if ss.rec != nil {
		ss.rec.AddSpecialEvent(eventType, details)
	}
}

// SetInput sets the stdin source.
func (ss *SSHSession) SetInput(stdIn io.Reader) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.StdIn = stdIn
}

// SetOutput sets the stdout target.
func (ss *SSHSession) SetOutput(stdOut, stdErr io.Writer) {
	ss.rec.SetPassthrough(stdOut, stdErr)
}

// SetEnv sets the environment variables to be used.
func (ss *SSHSession) SetEnv(vars map[string]string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.EnvVars = vars
}

// SetTerm sets the terminal dimensions on connection.
func (ss *SSHSession) SetTerm(height, width int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.PTYConfig = &PTYConfig{
		Term:         "xterm", // Can't find what this value *does*, so it is hardcoded.
		Height:       height,
		Width:        width,
		TerminalMode: DefaultTerminalMode,
	}
}

// SetWindowChange sets the channel used to update the window dimensions.
func (ss *SSHSession) SetWindowChange(winChC <-chan struct{ Height, Width int }) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.WinCh = winChC
}

// Signal is used to send a signal.
func (ss *SSHSession) Signal(s signal.Signal) error {
	return errors.New("unimplemented")
}

// Run runs the session and waits for it to complete.
func (ss *SSHSession) Run(ctx context.Context) (*recorder.Recorder, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ctx == nil {
		panic("nil context")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Stop the session if either context is cancelled.
	go func() {
		defer ss.finishFn()
		select {
		case <-ctx.Done():
			// Nothing to do.
			return
		case <-ss.parentCtx.Done():
			cancel()
			return
		}
	}()

	err := ss.ssh.RunCommand(ctx, ss.conf)
	ss.logger.Debugf("Finished run of command: %s", ss.conf.Command)
	return ss.rec, errors.Wrap(err, "run command error")
}

// Start starts the session in a sesparate goroutine.
//
// The returned Recorder pointer should not be dereferenced until after Wait
// completes.
func (ss *SSHSession) Start(ctx context.Context) (*recorder.Recorder, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ctx == nil {
		panic("nil context")
	}

	var cancel context.CancelFunc
	ss.ctx, cancel = context.WithCancel(ctx)

	// Stop the session if either context is cancelled.
	go func() {
		defer ss.finishFn()
		select {
		case <-ctx.Done():
			// Nothing to do.
			return
		case <-ss.parentCtx.Done():
			cancel()
			return
		case ss.errC <- ss.ssh.RunCommand(ctx, ss.conf):
			return
		}
	}()

	return ss.rec, nil
}

// Wait waits for the session to complete after calling Start.
//
// If Start has not been called, an error will be returned.
func (ss *SSHSession) Wait() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.ctx == nil {
		return errors.New("no command running")
	}

	return <-ss.errC
}
