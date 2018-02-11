// Package recursivelistener implements a Listener that closes its connections
// when it is closed.
//
// This is useful in tests for shutting down code that will run forever until
// an error is encountered from the connection(s) that it is using.
package recursivelistener

import (
	"bytes"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// New returns a new Listener.
func New(l net.Listener) *Listener {
	return &Listener{
		Listener: l,
	}
}

// Listener is a net.Listener implementation that closes connections that were
// accepted from it whenever it is closed.
type Listener struct {
	net.Listener
	conns []net.Conn
	mu    sync.Mutex
}

// Accept accepts a connection.
func (l *Listener) Accept() (net.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	c, err := l.Listener.Accept()
	if err != nil {
		return c, err
	}

	l.conns = append(l.conns, c)
	return c, nil
}

// Close closes the Listener and all of the connections that were accepted
// from the Listener.
//
// This call will panic if it takes too long to close all of the connections.
func (l *Listener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	errC := make(chan error)
	go func() {
		// Close all of the net.Conns opened by this Listener.
		var errs []error
		for i := range l.conns {
			err := l.conns[i].Close()
			if err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			// Preserve stack trace.
			if len(errs) == 1 {
				errC <- errors.Wrap(errs[0], "unable to recursively close Listener")
				return
			}
			var buf bytes.Buffer
			buf.WriteString("conn errors: [")
			for i := range errs {
				buf.WriteString(errs[i].Error())
				if i == len(errs)-1 {
					buf.WriteString("]")
				} else {
					buf.WriteString("; ")
				}
			}
			errC <- errors.New(buf.String())
			return
		}
		errC <- l.Listener.Close()
	}()

	a := time.NewTimer(5 * time.Second)
	defer a.Stop()

	select {
	case err := <-errC:
		return err
	case <-a.C:
		panic("timeout attempting to close connections")
	}
}
