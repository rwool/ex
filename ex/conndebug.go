package ex

import (
	"context"
	"errors"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/rwool/ex/log"
)

type debugDialer struct {
	Dialer

	logger                  log.Logger
	suppressErrs            uint32
	suppressCloseAfterClose bool
}

func (dd *debugDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	c, err := dd.Dialer.DialContext(ctx, network, address)
	c = &debugConn{
		Conn:                    c,
		logger:                  dd.logger,
		suppressErrs:            &dd.suppressErrs,
		suppressCloseAfterClose: dd.suppressCloseAfterClose,
	}
	return c, err
}

// suppressErrors prevents the logging of errors from closing connections.
func (dd *debugDialer) suppressErrors() {
	atomic.StoreUint32(&dd.suppressErrs, 1)
}

var errCloseAfterClosed = errors.New("close call on closed connection")

type debugConn struct {
	net.Conn
	mu           sync.Mutex
	isClosed     bool
	logger       log.Logger
	suppressErrs *uint32
	// Make close attempts after closing not
	// return an error.
	suppressCloseAfterClose bool
}

func (dc *debugConn) Close() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Do not log errors.
	if atomic.LoadUint32(dc.suppressErrs) == 1 {
		return dc.Conn.Close()
	}

	if dc.isClosed {
		if dc.suppressCloseAfterClose {
			return nil
		}
		dc.logger.Errorf("Close attempted with closed connection:\n%s\n", string(debug.Stack()))
		return errCloseAfterClosed
	}

	err := dc.Conn.Close()
	if err != nil {
		dc.logger.Errorf("Error closing connection: %+v", err)
	} else {
		dc.isClosed = true
	}
	return err
}
