// Package sshtarget provides support for managing a client SSH connection.
package sshtarget

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/pkg/errors"

	"github.com/rwool/ex/log"

	"golang.org/x/crypto/ssh"
)

// SSH is a wrapper around the SSH client.
type SSH struct {
	sshClient *ssh.Client
}

// Close closes the connection to the SSH server.
func (s *SSH) Close() error {
	return s.sshClient.Close()
}

// HostKeyCallback is a function for handling host keys.
type HostKeyCallback = ssh.HostKeyCallback

var (
	// InsecureIgnoreHostKey ignores the host key.
	InsecureIgnoreHostKey = ssh.InsecureIgnoreHostKey
	// FixedHostKey uses a single, fixed public key.
	FixedHostKey = ssh.FixedHostKey
)

// NewSSH creates a new SSH connection with the given configuration.
func NewSSH(ctx context.Context, logger log.Logger, conn net.Conn, address string,
	keyCallback HostKeyCallback, username string, auths []Authorizer) (*SSH, error) {
	sshAuths := make([]ssh.AuthMethod, 0, len(auths))
	for i, v := range auths {
		if v == nil {
			panic(fmt.Sprintf("nil authorizer given at index %d", i))
		}
		sshAuths = append(sshAuths, v.GetAuthMethod())
	}

	sshConf := ssh.ClientConfig{
		User:            username,
		Auth:            sshAuths,
		HostKeyCallback: keyCallback,
	}

	// Error used to indicate that the ssh connection closed due to the
	// underlying net.Conn being closed (due to the context being done).
	var contextError error

	sshConnCreatedC := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	// Close the underlying connection if the context is cancelled. If/Until explicit
	// cancellation is added to the SSH library, there does not appear to be a better
	// way of doing this.
	go func() {
		select {
		case <-ctx.Done():
			logger.Debug("closing net.Conn for SSH due to context being done")
			contextError = errors.Wrap(ctx.Err(), "SSH connection cancelled")
			err := conn.Close()
			if err != nil {
				logger.Warnf("error attempting to close network connection for SSH: %+v", err)
			}
			wg.Done()
		case <-sshConnCreatedC:
			wg.Done()
			return
		}
	}()

	logger.Debugf("attempting to connect to %q (network: %s) with username %q", address, conn.RemoteAddr(), sshConf.User)
	sshConn, channels, requests, err := ssh.NewClientConn(conn, address, &sshConf)
	close(sshConnCreatedC)
	wg.Wait()
	if err != nil {
		if contextError != nil {
			return nil, contextError
		}
		return nil, errors.Wrap(err, "failed to create SSH client connection")
	}

	return &SSH{
		sshClient: ssh.NewClient(sshConn, channels, requests),
	}, nil
}

// PTYConfig ised used to configure the PTY settings when connecting via SSH.
type PTYConfig struct {
	Term         string
	Height       int
	Width        int
	TerminalMode ssh.TerminalModes
}

// WindowDims contains the dimensions of the window.
type WindowDims struct {
	Height int
	Width  int
}

// RunConfig contains the configuration for running a command over the SSH
// connection.
type RunConfig struct {
	StdIn  io.Reader
	StdOut io.Writer
	StdErr io.Writer

	PTYConfig *PTYConfig

	// WinCh is an optional channel that will be received from that will update
	// the window dimensions dynamically.
	WinCh <-chan struct{ Height, Width int }

	AsyncErrLogger func(error)

	Command string
	EnvVars map[string]string

	PreRunFunc  func()
	PostRunFunc func()
}

// DefaultTerminalMode is the default mode that will be set for the terminal.
var DefaultTerminalMode = ssh.TerminalModes{
	ssh.ECHO:          1,
	ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
	ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
}

func noOpIfNil(fn *func()) {
	if *fn == nil {
		*fn = func() {}
	}
}

// RunCommand runs a command with the given configuration.
func (s *SSH) RunCommand(ctx context.Context, config RunConfig) error {
	noOpIfNil(&config.PreRunFunc)
	noOpIfNil(&config.PostRunFunc)

	sess, err := s.sshClient.NewSession()
	if err != nil {
		return errors.Wrap(err, "unable to create session")
	}

	sess.Stdin = config.StdIn
	sess.Stdout = config.StdOut
	sess.Stderr = config.StdErr

	for k, v := range config.EnvVars {
		err = sess.Setenv(k, v)
		if err != nil {
			return errors.Wrap(err, "unable to set environment variable for session")
		}
	}

	if config.PTYConfig != nil {
		err = sess.RequestPty(config.PTYConfig.Term,
			config.PTYConfig.Height,
			config.PTYConfig.Width,
			DefaultTerminalMode)
		if err != nil {
			return errors.Wrap(err, "unable to acquire PTY")
		}
	}

	doneC := make(chan struct{})
	if config.WinCh != nil {
		go func() {
			var logger func(error)
			if config.AsyncErrLogger == nil {
				logger = func(error) {}
			} else {
				logger = config.AsyncErrLogger
			}
			select {
			case dims := <-config.WinCh:
				err = sess.WindowChange(dims.Height, dims.Width)
				if err != nil {
					logger(errors.Wrap(err, "unable to update window dimensions"))
				}
			case <-doneC:
				return
			}
		}()
	}
	defer close(doneC)

	config.PreRunFunc()
	defer config.PostRunFunc()

	if config.Command == "" {
		// Requesting Shell.
		err = sess.Shell()
		if err != nil {
			return errors.Wrap(err, "unable to create shell via SSH")
		}
	} else {
		// Running command.
		err = sess.Run(config.Command)
		if err != nil {
			return errors.Wrap(err, "unable to run command via SSH")
		}
	}

	return nil
}
