package ex

import (
	"context"
	errors2 "errors"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/rwool/ex/ex/session"
	"github.com/rwool/ex/log"
)

// SSHTarget is a generic connection to a system for executing commands.
type SSHTarget struct {
	user      string
	host      string
	port      uint16
	dialer    Dialer
	logger    log.Logger
	auths     []session.Authorizer
	hostKeyCB session.HostKeyCallback

	mu sync.Mutex

	client *session.SSH

	sessions []*sshSession

	sessionCtx    context.Context
	sessionCancel context.CancelFunc

	sessionWG sync.WaitGroup

	isClosed bool
}

// NewSSHTarget creates a new connection with an SSH server.
func NewSSHTarget(ctx context.Context, logger log.Logger, dialer Dialer,
	host string, port uint16, opts []Option, username string, auths []session.Authorizer) (*SSHTarget, error) {
	if logger == nil {
		panic("nil logger")
	}
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	if len(auths) == 0 {
		panic("no authorizers given")
	}

	var hkc session.HostKeyCallback
	for _, v := range opts {
		switch v.(type) {
		case session.HostKeyCallback:
			hkc = v.(session.HostKeyCallback)
		}
	}
	hkc = session.InsecureIgnoreHostKey()

	c := &SSHTarget{
		user:      username,
		host:      host,
		port:      port,
		dialer:    dialer,
		logger:    logger,
		auths:     auths,
		hostKeyCB: hkc,
	}

	// Distinct from the context passed into this function.
	// The context passed into this function is for cancellation of the client
	// connection, while this context is used for cancellation of this
	// connection after it has been made, as well as all sessions from this
	// connection.
	c.sessionCtx, c.sessionCancel = context.WithCancel(context.Background())

	err := c.getClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get SSH connection")
	}

	return c, nil
}

// Command creates a command that can be run on the SSHTarget.
func (st *SSHTarget) Command(cmd string, args ...string) Command {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.sessionWG.Add(1)
	as := newSSHSession(st.sessionCtx, func() {
		st.logger.Debugf("Finishing up command: %s", cmd)
		st.sessionWG.Done()
	})

	as.rec = NewRecorder()
	as.rec.setCommand(cmd, args...)
	as.logger = st.logger
	as.conf.Command = as.rec.command()
	as.conf.AsyncErrLogger = func(e error) {
		as.logger.Errorf("Error in SSH session: %+v", e)
	}
	as.conf.PreRunFunc = as.rec.startTiming
	as.rec.setOutput(&as.conf.StdOut, &as.conf.StdErr)
	as.ssh = st.client
	as.errC = make(chan error)

	st.sessions = append(st.sessions, as)

	return as
}

// Option is an an additional setting that may be made when creating an
// SSHTarget.
type Option interface{}

// WindowDims contains the dimensions of the window.
type WindowDims = session.WindowDims

// getClient gets an SSH connection by connecting to an SSH server.
func (st *SSHTarget) getClient(ctx context.Context) error {
	address := net.JoinHostPort(st.host, strconv.Itoa(int(st.port)))
	conn, err := st.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.Wrap(err, "unable to dial remote host")
	}

	client, err := session.NewSSH(ctx, st.logger, conn, address, st.hostKeyCB, st.user, st.auths)
	if err != nil {
		return errors.Wrap(err, "unable to get SSH session")
	}

	st.client = client
	return nil
}

// Close closes the SSH target and all related commands.
// Blocks until all commands have been closed.
func (st *SSHTarget) Close() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.isClosed {
		return nil
	}

	// Close all of the SSH sessions under this target.
	hp := net.JoinHostPort(st.host, strconv.Itoa(int(st.port)))
	st.logger.Debugf("Closing SSH target to %s", hp)
	st.sessionCancel()
	st.sessionWG.Wait()

	// Close the underlying connection used for the SSH connection.
	// Note that this will likely cause multiple calls to be made with the Close
	// method of the Conn.
	// This is because calling Close makes a call, which indirectly causes the
	// SSH handshake and mux goroutines to also make calls to Close.
	st.client.Close()
	st.isClosed = true
	st.logger.Debugf("No errors closing SSH target: %s", hp)

	return nil
}

// TermConfig represents the configuration that will be used for managing the
// terminal's dimensions.
type TermConfig struct {
	Term   string
	Width  int
	Height int
	WinCh  <-chan WindowDims
}

// IOConfig represents the config for the input and output for the target.
type IOConfig struct {
	StdIn io.Reader

	StdOutPassthrough io.Writer
	StdErrPassthrough io.Writer
}

// ErrPTYAllocFail indicates an error allocating a PTY.
var ErrPTYAllocFail = errors2.New("unable to allocate PTY")

// ErrNoSSHConnection indicates that there was no SSH connection.
var ErrNoSSHConnection = errors2.New("no SSH connection")

// HostKeyValidationOption returns an option to set the use of a host key
// callback.
func HostKeyValidationOption(callback session.HostKeyCallback) Option {
	return callback
}
