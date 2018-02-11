// Package testlogger implements the redirection of logging output to
// test-specific logs.
package testlogger

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/rwool/ex/log"
)

// Buffer is a bytes.Buffer that is thread-safe for most operations.
type Buffer struct {
	bytes.Buffer
	mu sync.Mutex
}

// WriteString writes out s into the buffer.
func (b *Buffer) WriteString(s string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.WriteString(s)
}

// WriteByte writes out c into the buffer.
func (b *Buffer) WriteByte(c byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.WriteByte(c)
}

// Write writes the bytes from p into the buffer.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

// WriteTo writes from the buffer into w until there is no more data in the
// buffer or a non-io.EOF error is encountered.
func (b *Buffer) WriteTo(w io.Writer) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.WriteTo(w)
}

// Read reads from the buffer into p.
func (b *Buffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Read(p)
}

// ReadFrom reads from r until an error is returned. If the error is io.EOF,
// then this function will return nil.
func (b *Buffer) ReadFrom(r io.Reader) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.ReadFrom(r)
}

// Bytes returns a slice of length b.Len() that holds the unread portion of the
// buffer.
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Bytes()
}

// String returns a string of the unread portion of the buffer.
func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

// BytesCopy returns a copy of the bytes of the unread portion of the buffer.
func (b *Buffer) BytesCopy() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	currentBytes := b.Buffer.Bytes()
	out := make([]byte, len(currentBytes), len(currentBytes))

	copy(out, currentBytes)

	return out
}

// testWriter writes to the io.Writer and logs to the testing.TB.
type testWriter struct {
	tb testing.TB
	w  io.Writer
	mu sync.Mutex
}

// Write writes p into the testWriter.
func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.tb.Helper()
	//tw.mu.Lock()
	//defer tw.mu.Unlock()

	written, err := tw.w.Write(p)
	if err != nil {
		return written, err
	}

	tw.tb.Log(string(p))

	return written, nil
}

// NewTestLogger creates a logger that stores log output in a buffer and outputs
// to the test/benchmark log output.
func NewTestLogger(tb testing.TB, level log.Level) (log.Logger, *Buffer) {
	buf := &Buffer{}

	tw := &testWriter{
		tb: tb,
		w:  buf,
	}

	return log.NewLogger(tw, level), buf
}
