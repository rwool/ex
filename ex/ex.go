// Package ex implements a tool for managing multiple connections to different
// systems and performing actions with them.
package ex

import (
	"container/list"
	"context"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sync"

	"github.com/pkg/errors"

	"github.com/rwool/ex/ex/internal/recorder"
	"github.com/rwool/ex/ex/internal/sshtarget"
	"github.com/rwool/ex/log"
)

// Dialer is the interface that wraps the dial method.
//
// Primarily used for abstracting out possible dialer implementations as there
// is no dialer interface in the standard library.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Target is a specific instance of a connection to a system.
//
// Two targets that point to the same system are not necessarily the same as
// they may have different internal state.
//
// Targets are capable of running commands on the system they target.
// A single target can support running multiple commands at once with the
// same configuration.
type Target interface {
	Command(cmd string, args ...string) Command
}

// Ex handles connections to multiple systems and provides ways of manipulating
// them.
type Ex struct {
	// mu is for locking everything that doesn't have a dedicated Locker.
	mu          sync.Mutex
	connections list.List
	dialer      Dialer
	logger      log.Logger
	doneC       chan struct{}

	stdOut io.Writer
	stdErr io.Writer

	nameToTargetsMu sync.RWMutex
	nameToTargets   map[string]Target

	recordingsMu      sync.RWMutex
	recorderC         chan *recorder.Recorder
	completedCommands []*recorder.Recorder
}

// New creates a new Ex object for executing commands remotely.
//
// If the given logger is nil, then a logger will be created that writes to
// stderr.
//
// Both of the writers given may be nil.
func New(logger log.Logger, stdOut, stdErr io.Writer) *Ex {
	if logger == nil {
		logger = log.NewLogger(os.Stderr, log.Warn)
	}
	if stdOut == nil {
		stdOut = ioutil.Discard
	}
	if stdErr == nil {
		stdErr = ioutil.Discard
	}

	r := &Ex{
		doneC: make(chan struct{}),

		logger: logger,
		stdOut: stdOut,
		stdErr: stdErr,

		nameToTargets: make(map[string]Target),

		recorderC: make(chan *recorder.Recorder),
	}
	r.SetDialer(&net.Dialer{})

	go func() {
		for {
			select {
			case rec := <-r.recorderC:
				r.recordingsMu.Lock()
				r.completedCommands = append(r.completedCommands, rec)
				// TODO Sort the slice here.
				r.recordingsMu.Unlock()
			// TODO Add Close case.
			case <-r.doneC:
				return
			}
		}
	}()

	return r
}

// SetDialer sets the dialer that will be used for all connections to remote
// systems.
func (r *Ex) SetDialer(d Dialer) {
	if d == nil {
		panic("nil dialer")
	}

	r.mu.Lock()
	r.dialer = &debugDialer{
		Dialer:                  d,
		logger:                  r.logger,
		suppressCloseAfterClose: true,
	}
	r.mu.Unlock()
}

// GetTarget gets the target with the given name, returning nil if none is
// found.
func (r *Ex) GetTarget(name string) Target {
	r.nameToTargetsMu.RLock()
	defer r.nameToTargetsMu.RUnlock()

	return r.nameToTargets[name]
}

// SSHTargetConfig contains the options for creating an SSH target.
type SSHTargetConfig struct {
	// Name of this connection in Ex.
	Name string
	// Host is the host that will be connected to.
	Host string
	// Port is the port of the host to connect to.
	Port uint16
	// User is the name of the user that the connection will be made with.
	User string
	// Auths is a list of the authorization methods that will be used upon
	// connection.
	Auths []SSHAuthorizer
	// HostKeyCallback is a function that is called to verify a host key.
	HostKeyCallback SSHHostKeyCallback
}

type SSHCommand struct {
	*sshtarget.SSHSession
}

// Run runs the session and waits for it to complete.
func (s *SSHCommand) Run(ctx context.Context) (Recorder, error) {
	return s.SSHSession.Run(ctx)
}

// Start starts the session in a sesparate goroutine.
//
// The returned Recorder pointer should not be dereferenced until after Wait
// completes.
func (s *SSHCommand) Start(ctx context.Context) (Recorder, error) {
	return s.SSHSession.Start(ctx)
}

// SSHTarget adapts the internal SSHTarget to the Target interface.
//
// This is necessary due to Go not having covariance.
type SSHTarget struct {
	*sshtarget.SSHTarget
}

// Command runs a command with the SSHTarget.
func (s *SSHTarget) Command(cmd string, args ...string) Command {
	t := s.SSHTarget.Command(cmd, args...)
	return &SSHCommand{SSHSession: t}
}

// NewSSHTarget creates an SSH target to the given system.
func (r *Ex) NewSSHTarget(ctx context.Context, conf *SSHTargetConfig) (Target, error) {
	r.nameToTargetsMu.Lock()
	defer r.nameToTargetsMu.Unlock()

	if conf.HostKeyCallback == nil {
		return nil, errors.New("no host key callback")
	}

	if _, ok := r.nameToTargets[conf.Name]; ok {
		return nil, errors.New("target already exists with the given name")
	}

	hkcOpt := sshtarget.HostKeyValidationOption(conf.HostKeyCallback)
	target, err := sshtarget.New(ctx,
		r.logger,
		r.dialer,
		conf.Host,
		conf.Port,
		[]sshtarget.Option{hkcOpt},
		conf.User,
		authConvert(conf.Auths))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create SSH target")
	}

	t := &SSHTarget{SSHTarget: target}
	r.nameToTargets[conf.Name] = t
	r.logger.Debugf("Added SSH target: %s", conf.Name)

	return t, nil
}

// Close closes all currently open connections.
func (r *Ex) Close() error {
	r.nameToTargetsMu.Lock()
	defer r.nameToTargetsMu.Unlock()

	// Suppressing errors, if possible.
	// This is to work around some otherwise unavoidable errors and log
	// messages that can come about after closing connections.
	if v, ok := r.dialer.(*debugDialer); ok {
		v.suppressErrors()
	}

	for k, v := range r.nameToTargets {
		if c, ok := v.(io.Closer); ok {
			err := c.Close()
			if err != nil {
				return errors.Wrap(err, "unable to complete closing Ex")
			}
			r.logger.Debugf("No errors closing target: %s", k)
			delete(r.nameToTargets, k)
		} else {
			r.logger.Errorf("Target %s missing Close method", k)
		}
	}

	close(r.doneC)
	return nil
}
