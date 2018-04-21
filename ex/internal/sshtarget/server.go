package sshtarget

import (
	"crypto/rand"
	"crypto/rsa"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/kballard/go-shellquote"
	"github.com/rwool/ex/log"
	"github.com/rwool/ex/test/helpers/clientserverpair"
	"github.com/rwool/ex/test/helpers/recursivelistener"
	ssh2 "golang.org/x/crypto/ssh"
)

// SSH server for testing only. Exported due to use in multiple packages.

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

// NewSSHServer creates an SSH server for testing against.
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
