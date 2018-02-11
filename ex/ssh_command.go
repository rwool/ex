package ex

import (
	"context"
	errors2 "errors"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/rwool/ex/log"
	"github.com/rwool/ex/ex/session"
)

// ErrCancelledByTarget indicates command was cancelled indirectly by the
// target.
var ErrCancelledByTarget = errors2.New("cancelled by target")

type sshSession struct {
	ssh    *session.SSH
	conf   session.RunConfig
	rec    *Recorder
	logger log.Logger

	mu sync.Mutex

	errC chan error

	parentCtx, ctx context.Context

	finishFn func()
}

func newSSHSession(ctx context.Context, finishFn func()) *sshSession {
	if ctx == nil {
		panic("nil context")
	}

	ss := &sshSession{
		parentCtx: ctx,
	}

	if finishFn == nil {
		ss.finishFn = func() {}
	} else {
		ss.finishFn = finishFn
	}

	return ss
}

func (ss *sshSession) LogEvent(eventType string, details interface{}) {
	if ss.rec != nil {
		ss.rec.AddSpecialEvent(eventType, details)
	}
}

func (ss *sshSession) SetInput(stdIn io.Reader) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.StdIn = stdIn
}

func (ss *sshSession) SetOutput(stdOut, stdErr io.Writer) {
	ss.rec.setPassthrough(stdOut, stdErr)
}

func (ss *sshSession) SetEnv(vars map[string]string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.EnvVars = vars
}

func (ss *sshSession) SetTerm(height, width int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.PTYConfig = &session.PTYConfig{
		Term:         "xterm", // Can't find what this value *does*, so it is hardcoded.
		Height:       height,
		Width:        width,
		TerminalMode: session.DefaultTerminalMode,
	}
}

func (ss *sshSession) SetWindowChange(winChC <-chan struct{ Height, Width int }) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.conf.WinCh = winChC
}

func (ss *sshSession) Signal(s Signal) error {
	return errors.New("unimplemented")
}

func (ss *sshSession) Run(ctx context.Context) (*Recorder, error) {
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

func (ss *sshSession) Start(ctx context.Context) (*Recorder, error) {
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

func (ss *sshSession) Wait() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.ctx == nil {
		return errors.New("no command running")
	}

	return <-ss.errC
}
