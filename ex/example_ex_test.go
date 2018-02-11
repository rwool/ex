package ex_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/rwool/ex/ex"
	"github.com/rwool/ex/ex/session"
	"github.com/rwool/ex/log"
	"sync"
)

// setupServer sets up an in-process SSH server to connect to.
//
// Only meant for demonstration purposes.
func setupServer() (sL net.Listener, ip string, port int) {
	ssh.Handle(func(s ssh.Session) {
		fmt.Fprintln(s, "test")
		s.Exit(0)
	})
	var (
		l     net.Listener
		lIP   string
		lPort int
		doneC = make(chan struct{})
		errC  = make(chan error)
		wg sync.WaitGroup // To prevent data race on return values.
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
		wg.Done()
		err = ssh.Serve(l,
			nil,
			ssh.PasswordAuth(func(user, password string) bool {
				// For example only, this is not secure.
				return user == "test" && password == "password123"
			}))
		select {
		case <-doneC:
			return
		default:
			panic(err)
		}
	}()
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

	select {
	case err := <-errC:
		panic(err)
	case <-timer.C:
		close(doneC)
	}
	wg.Wait()
	return l, lIP, lPort
}

func Example() {
	// For example client to connect to.
	sL, ip, port := setupServer()

	l := log.NewLogger(os.Stderr, log.Warn)
	e := ex.New(l, os.Stdout, os.Stderr)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	defer sL.Close()

	target, err := e.NewSSHTarget(ctx,
		"Server 1",
		ip,
		uint16(port),
		"test",
		[]session.Authorizer{session.PasswordAuth("password123")})
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
