package ex_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"
	ssh2 "golang.org/x/crypto/ssh"

	"github.com/rwool/ex/test/helpers/recursivelistener"

	"github.com/stretchr/testify/require"

	"github.com/rwool/ex/ex"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"github.com/gliderlabs/ssh"
	"github.com/kballard/go-shellquote"
	"github.com/rwool/ex/test/helpers/clientserverpair"

	"crypto/rand"
	"crypto/rsa"

	"github.com/rwool/ex/ex/session"
	"github.com/rwool/ex/log"
	"github.com/rwool/ex/test/helpers/testlogger"
)

type commandOutput struct {
	Output string
	Code   int
}

var commandMap = map[string]commandOutput{
	"whoami": {
		Output: "test\n",
		Code:   0,
	},
	"sh -c 'echo $SHELL'": {
		Output: "/bin/bash\n",
		Code:   0,
	},
	"doesNotExist": {
		// TODO: This is probably not like the actual output, if any.
		Output: "-bash: doesNotExist: command not found\n",
		Code:   127,
	},
}

func NewSSHServer(logger log.Logger) (d clientserverpair.Dialer, pubKey ssh2.PublicKey, stop func()) {
	ssh.Handle(func(s ssh.Session) {
		if len(s.Command()) == 0 {
			panic("unimplemented")
		} else {
			// Exec a command.
			cmd := shellquote.Join(s.Command()...)
			out := commandMap[cmd]
			s.Write([]byte(out.Output))
			s.Exit(out.Code)
		}
	})

	d, l := clientserverpair.New(&clientserverpair.PipeCSPairConfig{
		Logger: logger,

		ClientReadDebug:  nil,
		ClientWriteDebug: nil,
		ServerReadDebug:  nil,
		ServerWriteDebug: nil,
	})
	li := recursivelistener.New(l)
	logger.Debug("created dialer/listener pair")
	errC := make(chan error)
	cancelC := make(chan struct{})

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	signer, err := ssh2.NewSignerFromKey(key)
	if err != nil {
		panic(err)
	}

	go func() {
		logger.Debug("About serve SSH")
		srv := &ssh.Server{
			PasswordHandler: func(user, password string) bool {
				if user == "test" && password == "Password123" {
					return true
				}
				return false
			},
			HostSigners: []ssh.Signer{signer.(ssh.Signer)},
		}
		err = srv.Serve(li)
		select {
		case errC <- err:
		case <-cancelC:
		}
	}()

	// Wait for a few milliseconds to ensure the server started without errors.
	select {
	case <-time.After(50 * time.Millisecond):
	case err := <-errC:
		panic(err)
	}
	logger.Debug("No error attempting to start SSH server")

	return d, signer.PublicKey(), func() { close(cancelC); l.Close() }
}

func TestConnectionSSH(t *testing.T) {
	defer goroutinechecker.New(t)()

	logger, _ := testlogger.NewTestLogger(t, log.Warn)
	dialer, hostKey, stopServer := NewSSHServer(logger)
	defer func() {
		stopServer()
		time.Sleep(50 * time.Millisecond)
	}()
	if v, ok := dialer.(io.Closer); ok {
		defer v.Close()
	}

	ctx, canceller := context.WithTimeout(context.Background(), 5*time.Second)
	defer canceller()

	validateRecording := func(r *ex.Recorder, has string) {
		buf := &bytes.Buffer{}
		err := r.Replay(buf, buf, 0)
		assert.NoError(t, err)
		assert.Equal(t, has, buf.String())
	}

	hkOpt := ex.HostKeyValidationOption(session.FixedHostKey(hostKey))
	c, err := ex.NewSSHTarget(ctx, logger, dialer, "127.0.0.1", 22,
		[]ex.Option{hkOpt}, "test",
		[]session.Authorizer{session.PasswordAuth("Password123")})
	require.NoError(t, err, "error getting SSH target")

	rec, err := c.Command("whoami").Run(ctx)
	assert.NoError(t, err)
	validateRecording(rec, "test\n")

	rec, err = c.Command("doesNotExist").Run(ctx)
	assert.EqualError(t, errors.Cause(err), "Process exited with status 127")
	validateRecording(rec, "-bash: doesNotExist: command not found\n")

	assert.NoError(t, c.Close(), "unexpected error from SSH target close")
}
