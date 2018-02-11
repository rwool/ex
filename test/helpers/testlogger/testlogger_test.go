package testlogger_test

import (
	"testing"

	"github.com/rwool/ex/log"
	"github.com/rwool/ex/test/helpers/testlogger"
	"github.com/stretchr/testify/assert"
)

func TestNewTestLogger(t *testing.T) {
	// Creating a new testing object to prevent unnecessary output.
	t2 := &testing.T{}
	l, buf := testlogger.NewTestLogger(t2, log.Debug)
	l.Debug("1")
	l.Warn("2")
	l.Error("3")
	assert.True(t, buf.Len() > 0, "no data in log buffer")
}

func TestBuffer(t *testing.T) {
	// Intended to be run with race detector.
	buf := &testlogger.Buffer{}
	for i := 0; i < 50; i++ {
		go func() {
			buf.Write([]byte("123"))
		}()
		go func() {
			outBuf := make([]byte, 20)
			buf.Read(outBuf)
		}()
	}
}
