package ex_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"crypto/rsa"

	"crypto/rand"

	"sync"

	"github.com/gliderlabs/ssh"
	"github.com/rwool/ex/ex"
	"github.com/rwool/ex/ex/session"
	"github.com/rwool/ex/log"
	ssh2 "golang.org/x/crypto/ssh"
)

// setupServer sets up an in-process SSH server to connect to.
//
// Only meant for demonstration purposes.
func setupServer() (sL net.Listener, ip string, port int, pubKey ssh.PublicKey) {
	ssh.Handle(func(s ssh.Session) {
		fmt.Fprintln(s, "test")
		s.Exit(0)
	})
	var (
		l      net.Listener
		lIP    string
		lPort  int
		signer ssh2.Signer
		errC   = make(chan error, 1)
		// WaitGroup to know when to start error timer. Key
		// generation can take longer than the timeout duration, so without
		// this, the test can fail.
		wg sync.WaitGroup
	)
	wg.Add(1)

	go func() {
		var err error
		l, err = net.Listen("tcp", ":")
		if err != nil {
			panic(err)
		}
		tcpAddr := l.Addr().(*net.TCPAddr)
		lIP = tcpAddr.IP.String()
		lPort = tcpAddr.Port
		lPort = l.Addr().(*net.TCPAddr).Port
		priv, err := rsa.GenerateKey(rand.Reader, 1024)
		if err != nil {
			panic(err)
		}
		signer, err = ssh2.NewSignerFromKey(priv)
		if err != nil {
			panic(err)
		}
		wg.Done()

		srv := &ssh.Server{
			Handler: nil,
			PasswordHandler: func(user, password string) bool {
				// For example only, this is not secure.
				return user == "test" && password == "password123"
			},
		}
		srv.AddHostKey(signer.(ssh.Signer))
		srv.Serve(l)
		errC <- ssh.Serve(l,
			nil,
			ssh.PasswordAuth(func(user, password string) bool {
				// For example only, this is not secure.
				return user == "test" && password == "password123"
			}))
	}()
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

	wg.Wait()
	select {
	case err := <-errC:
		panic(err)
	case <-timer.C:
	}
	return l, lIP, lPort, signer.PublicKey()
}

func Example() {
	// For example client to connect to.
	sL, ip, port, pubKey := setupServer()

	l := log.NewLogger(os.Stderr, log.Warn)
	e := ex.New(l, os.Stdout, os.Stderr)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	defer sL.Close()

	target, err := e.NewSSHTarget(ctx, &ex.SSHTargetConfig{
		Name: "Server 1",
		Host: ip,
		Port: uint16(port),
		User: "test",
		Auths: []session.Authorizer{
			session.PasswordAuth("password123"),
		},
		HostKeyCallback: session.FixedHostKey(pubKey),
	})
	if err != nil {
		l.Errorf("Unable to connect to SSH server: %+v", err)
		os.Exit(1)
	}

	cmd := target.Command("whoami")
	cmd.SetOutput(os.Stdout, nil)
	_, err = cmd.Run(ctx)
	if err != nil {
		l.Errorf("Error running command: %+v", err)
		os.Exit(1)
	}

	err = e.Close()
	if err != nil {
		panic(err)
	}

	// Output:
	// test
}
