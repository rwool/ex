package clientserverpair_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/rwool/ex/log"
	"github.com/rwool/ex/test/helpers/testlogger"

	"github.com/rwool/ex/test/helpers/clientserverpair"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientServerPair(t *testing.T) {
	defer goroutinechecker.New(t)()

	// Note that this test is written mostly with synchronous code.
	// This will not work with net.Pipe as writes will block waiting for
	// corresponding reads.
	// This works assuming that buffering is used for writes and reads.
	logger, _ := testlogger.NewTestLogger(t, log.Warn)
	d, l := clientserverpair.New(&clientserverpair.PipeCSPairConfig{
		Logger: logger,
	})

	var clientToServerBuf, serverToClientBuf [1024]byte
	var sConn net.Conn
	var lErr error
	accepted := make(chan struct{})
	go func() {
		sConn, lErr = l.Accept()
		accepted <- struct{}{}
	}()

	c, err := d.DialContext(context.Background(), "tcp", "127.0.0.1:22")
	require.NoError(t, err)
	select {
	case <-accepted:
		require.NoError(t, lErr, "unexpected error reading accepting from Listener")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Server side failed to accept connection")
	}

	// Ensure the Conns are usable after closing the Listener.
	require.NoError(t, l.Close(), "failed to close Listener")

	_, err = c.Write([]byte("Some Text"))
	require.NoError(t, err)

	read, err := sConn.Read(clientToServerBuf[:])
	require.NoError(t, err)
	assert.Equal(t, "Some Text", string(clientToServerBuf[:read]))

	_, err = fmt.Fprint(sConn, "Hello")
	require.NoError(t, err)

	read, err = c.Read(serverToClientBuf[:])
	require.NoError(t, err)
	assert.Equal(t, "Hello", string(serverToClientBuf[:read]))

	err = l.Close()
	assert.NoError(t, err)

}
