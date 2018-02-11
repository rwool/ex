// Package blockingreader implements an io.Reader that can block the first
// read for an arbitrary amount of time.
package blockingreader

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"sync"
	"time"
)

// BlockingReader is a reader that blocks on read calls until a receive
// completes from a given channel.
type BlockingReader struct {
	once      sync.Once
	cancelErr error
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	r         io.Reader
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Read reads from the underlying reader once a receive completes from the
// channel.
func (br *BlockingReader) Read(p []byte) (int, error) {
	// Only block the first time.
	br.once.Do(func() {
		br.mu.Lock()
		defer br.mu.Unlock()

		<-br.ctx.Done()
		if br.ctx.Err() == context.Canceled {
			br.cancelErr = errCancelled
		}
	})
	br.mu.Lock()
	defer br.mu.Unlock()

	if br.cancelErr != nil {
		return 0, br.cancelErr
	}

	return br.r.Read(p)
}

var errCancelled = errors.New("cancelled")

// Cancel stops the BlockingReader early and makes all reads return.
func (br *BlockingReader) Cancel() {
	br.cancel()
}

// NewBlockingReader creates a new BlockingReader.
func NewBlockingReader(allowAfter time.Duration, r io.Reader) *BlockingReader {
	ctx, cancelFunc := context.WithTimeout(context.Background(), allowAfter)
	return &BlockingReader{
		ctx:    ctx,
		r:      r,
		cancel: cancelFunc,
	}
}
