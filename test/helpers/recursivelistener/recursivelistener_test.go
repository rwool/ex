package recursivelistener_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/rwool/ex/log"
	"github.com/rwool/ex/test/helpers/clientserverpair"
	"github.com/rwool/ex/test/helpers/recursivelistener"
	"github.com/rwool/ex/test/helpers/testlogger"
)

func TestRecursiveListener(t *testing.T) {
	defer goroutinechecker.New(t)()

	l, _ := testlogger.NewTestLogger(t, log.Warn)

	dialer, listener := clientserverpair.New(&clientserverpair.PipeCSPairConfig{
		Logger: l,

		ClientReadDebug:  nil,
		ClientWriteDebug: nil,
		ServerReadDebug:  nil,
		ServerWriteDebug: nil,
	})

	errC := make(chan error)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for i := 0; i < 3; i++ {
			_, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:22")
			if err != nil {
				errC <- err
				return
			}
		}
	}()

	rl := recursivelistener.New(listener)
	acceptDoneC := make(chan struct{})
	go func() {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			rl.Close()
		case <-acceptDoneC:
		}
	}()

	var conns [3]net.Conn
	for i := range conns {
		var err error
		conns[i], err = rl.Accept()
		require.NoError(t, err, "error accepting connection")
	}
	close(acceptDoneC)

	// Close the Listener and check that the "child" connections were closed.
	require.NoError(t, rl.Close(), "unexpected error closing Listener")
	for i := range conns {
		assert.EqualError(t, conns[i].Close(), clientserverpair.ErrClosed.Error(),
			"did not get expected error from closing connection")
	}
}
