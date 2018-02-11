package clientserverpair

import (
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func newConnPair(bufSize int) (*pipeConn, *pipeConn) {
	b1 := bytes.NewBuffer(make([]byte, 0, bufSize))
	b2 := bytes.NewBuffer(make([]byte, 0, bufSize))
	var b1Mu, b2Mu sync.Mutex

	pc1 := &pipeConn{
		cToS:   b1,
		cToSMu: &b1Mu,
		sToC:   b2,
		sToCMu: &b2Mu,
	}
	pc2 := &pipeConn{
		cToS:   b2,
		cToSMu: &b2Mu,
		sToC:   b1,
		sToCMu: &b1Mu,
	}
	return pc1, pc2
}

type pipeConn struct {
	cToS io.Writer
	sToC io.Reader
	// Mutexes used because readers and writers are not assumed to be
	// thread-safe.
	cToSMu, sToCMu *sync.Mutex
	closed         uint32
}

func (pc *pipeConn) isClosed() bool {
	return atomic.LoadUint32(&pc.closed) == 1
}

func (pc *pipeConn) Read(b []byte) (n int, err error) {
	if pc.isClosed() {
		return 0, ErrClosed
	}

	pc.sToCMu.Lock()
	defer pc.sToCMu.Unlock()

	read, err := pc.sToC.Read(b)
	if err == io.EOF {
		return read, nil
	}
	return read, err
}

func (pc *pipeConn) Write(b []byte) (n int, err error) {
	if pc.isClosed() {
		return 0, ErrClosed
	}

	pc.cToSMu.Lock()
	defer pc.cToSMu.Unlock()

	return pc.cToS.Write(b)
}

// Close closes the connection.
//
// Any future calls to methods of this object will return ErrClosed.
func (pc *pipeConn) Close() error {
	if atomic.CompareAndSwapUint32(&pc.closed, 0, 1) {
		// Succeeded. First call to close the connection.
		return nil
	}
	// Failed. Not the first call to attempt to close the connection.
	return ErrClosed
}

func (pc *pipeConn) LocalAddr() net.Addr {
	panic("implement me")
}

func (pc *pipeConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 22,
	}
}

func (pc *pipeConn) SetDeadline(t time.Time) error {
	if pc.isClosed() {
		return ErrClosed
	}

	panic("implement me")
}

func (pc *pipeConn) SetReadDeadline(t time.Time) error {
	if pc.isClosed() {
		return ErrClosed
	}

	panic("implement me")
}

func (pc *pipeConn) SetWriteDeadline(t time.Time) error {
	if pc.isClosed() {
		return ErrClosed
	}

	panic("implement me")
}
