// Package clientserverpair provides a buffered, connected pair of dialers and
// listeners.
//
// This pair of objects differs from the net.Pipe implementation in that reads
// and writes are buffered and operations on them do not block, unless the
// respective internal buffer(s) is/are full.
package clientserverpair

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/rwool/ex/log"
)

var (
	// ErrClosed indicates that there was an attempt to use a closed connection.
	ErrClosed = errors.New("conn: use of closed connection")
	// ErrListenerClosed indicates that there was an attempt to use a closed
	// listener.
	ErrListenerClosed = errors.New("listener closed")
)

var (
	nextID int
	idMu   sync.Mutex
)

func getNextID() int {
	idMu.Lock()
	defer idMu.Unlock()

	id := nextID
	nextID++
	return id
}

type attempts struct {
	mu                           sync.Mutex
	cRead, cWrite, sRead, sWrite int
}

func (a *attempts) String() string {
	return fmt.Sprintf("(C: %d, %d; S %d, %d)", a.cRead, a.cWrite, a.sRead, a.sWrite)
}

var accessAttempts = map[int]*attempts{}

// DebugConn is a net.Conn implementation that can log its input and output.
type DebugConn struct {
	net.Conn

	isClient   bool
	id         int
	logger     log.Logger
	readDebug  RWDebugger
	writeDebug RWDebugger
	closed     uint32
}

func (dc *DebugConn) isClosed() bool {
	return atomic.LoadUint32(&dc.closed) == 1
}

// Read reads up to len(p) bytes from the connection.
func (dc *DebugConn) Read(p []byte) (int, error) {
	if dc.isClosed() {
		return 0, ErrClosed
	}

	n, err := dc.Conn.Read(p)

	if dc.readDebug != nil {
		dc.readDebug(dc.logger, true, dc.isClient, dc.id, p, n, err)
	}

	return n, err
}

// Write writes len(p) bytes from p to the connection.
func (dc *DebugConn) Write(p []byte) (int, error) {
	if dc.isClosed() {
		return 0, ErrClosed
	}

	n, err := dc.Conn.Write(p)

	if dc.writeDebug != nil {
		dc.readDebug(dc.logger, false, dc.isClient, dc.id, p, n, err)
	}

	return n, err
}

// Addr holds the information for the connection. This typically does not hold
// much meaning because this is not for a "real" connection.
type Addr struct {
	NetworkStr string
	StringStr  string
}

// Network returns the type of network that is used for the connection.
func (a *Addr) Network() string {
	return a.NetworkStr
}

// String returns the string form of the address.
func (a *Addr) String() string {
	return a.StringStr
}

// PipeListener is a net.Listener implementation that is paired up with a
// corresponding dialer.
//
// Accepting connections with this listener returns a net.Conn object that
// reads and writes from and to buffers that are shared with another net.Conn
// object that is used by client side of the connection.
type PipeListener struct {
	connC     <-chan net.Conn
	doneC     chan struct{}
	addr      *Addr
	closeOnce *sync.Once
}

// Accept accepts a connection. The returned net.Conn object is paired up with
// a corresponding client side net.Conn object that share a pair of read and
// write buffers.
func (pl *PipeListener) Accept() (c net.Conn, e error) {
	select {
	case conn := <-pl.connC:
		return conn, nil
	case <-pl.doneC:
		return nil, ErrListenerClosed
	}
}

// Close closes the listener.
func (pl *PipeListener) Close() error {
	pl.closeOnce.Do(func() { close(pl.doneC) })
	return nil
}

// Addr returns the address that is being listened on.
func (pl *PipeListener) Addr() net.Addr {
	return pl.addr
}

// Dialer is the interface that wraps the dial method.
//
// Primarily used for abstracting out possible dialer implementations as there
// is no dialer interface in the standard library.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// PipeDialer is a in memory dialer that opens net.Conn pipes in conjunction
// with Accept calls from the corresponding PipeListener.
type PipeDialer struct {
	connC chan<- net.Conn

	logger log.Logger

	clientReadDebug  RWDebugger
	clientWriteDebug RWDebugger

	serverReadDebug  RWDebugger
	serverWriteDebug RWDebugger
}

// DialContext creates a client side connection that is paired with the server
// side connection.
func (pd *PipeDialer) DialContext(_ context.Context, network, address string) (net.Conn, error) {
	c, s := newConnPair(1 << 10)

	id := getNextID()

	accessAttempts[id] = &attempts{}

	clientConn := &DebugConn{
		Conn:       c,
		logger:     pd.logger,
		isClient:   true,
		id:         id,
		readDebug:  pd.clientReadDebug,
		writeDebug: pd.clientWriteDebug,
	}

	serverConn := &DebugConn{
		Conn:       s,
		logger:     pd.logger,
		isClient:   false,
		id:         id,
		readDebug:  pd.serverReadDebug,
		writeDebug: pd.serverWriteDebug,
	}

	pd.connC <- serverConn

	return clientConn, nil
}

// RWDebugger is a function that can be used to debug read and write calls.
type RWDebugger func(logger log.Logger, isRead, isClient bool, pairID int, data []byte, processed int, err error)

// Example connection debugger.
//
//func updateAttempt(a *attempts, isRead, isClient bool) {
//	a.mu.Lock()
//	defer a.mu.Unlock()
//
//	if isRead {
//		if isClient {
//			a.cRead++
//		} else {
//			a.sRead++
//		}
//	} else {
//		if isClient {
//			a.cWrite++
//		} else {
//			a.sWrite++
//		}
//	}
//}
//
//func BasicDebugger(logger log.Logger, isRead, isClient bool, pairID int, data []byte, processed int, err error) {
//	updateAttempt(accessAttempts[pairID], isRead, isClient)
//
//	var clientServer string
//	if isClient {
//		clientServer = "client"
//	} else {
//		clientServer = "server"
//	}
//
//	var readWrite string
//	if isRead {
//		readWrite = "read"
//	} else {
//		readWrite = "write"
//	}
//
//	logger.Debugf("(%d) %s %s: %s %d bytes, err: %v, data:\n%s, Stack:\n%s",
//		pairID, clientServer, readWrite, accessAttempts[pairID].String(), processed, err, spew.Sdump(data), string(debug.Stack()))
//}

// PipeCSPairConfig contains configuration information for the creation of a
// Dialer/Listener pipe pair.
type PipeCSPairConfig struct {
	Logger           log.Logger
	ClientReadDebug  RWDebugger
	ClientWriteDebug RWDebugger
	ServerReadDebug  RWDebugger
	ServerWriteDebug RWDebugger
}

// New creates a new dialer and listener pipe pair.
//
// Connections returned from the dialer and listener will be connected via
// shared buffers.
func New(pcpc *PipeCSPairConfig) (*PipeDialer, *PipeListener) {
	connC := make(chan net.Conn)

	noOpIfNil := func(rwd RWDebugger) RWDebugger {
		if rwd == nil {
			return func(log.Logger, bool, bool, int, []byte, int, error) {}
		}
		return rwd
	}

	pd := &PipeDialer{
		connC: connC,

		logger: pcpc.Logger,

		clientReadDebug:  noOpIfNil(pcpc.ClientReadDebug),
		clientWriteDebug: noOpIfNil(pcpc.ClientWriteDebug),

		serverReadDebug:  noOpIfNil(pcpc.ServerReadDebug),
		serverWriteDebug: noOpIfNil(pcpc.ServerWriteDebug),
	}

	pl := &PipeListener{
		connC: connC,
		doneC: make(chan struct{}),
		addr: &Addr{
			NetworkStr: "pipe",
			StringStr:  "127.0.0.1:22",
		},
		closeOnce: &sync.Once{},
	}

	return pd, pl
}
