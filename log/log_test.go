package log_test

import (
	"bytes"
	"testing"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/rwool/ex/log"
	"github.com/stretchr/testify/assert"
)

func TestLoggerBufferWarn(t *testing.T) {
	defer goroutinechecker.New(t)()

	for _, level := range []log.Level{log.Debug, log.Warn, log.Error} {
		logBuf := bytes.Buffer{}
		logger := log.NewLogger(&logBuf, level)
		warnStr := "Test123Test"
		logger.Warn(warnStr)
		debugStr := "ABCD"
		logger.Debug(debugStr)
		errorStr := "XYZ"
		logger.Error(errorStr)

		has := func(s string) {
			assert.Contains(t, logBuf.String(), s)
		}

		notHas := func(s string) {
			assert.NotContains(t, logBuf.String(), s)
		}

		switch level {
		case log.Debug:
			has(debugStr)
			has(warnStr)
			has(errorStr)
		case log.Warn:
			notHas(debugStr)
			has(warnStr)
			has(errorStr)
		case log.Error:
			notHas(debugStr)
			notHas(warnStr)
			has(errorStr)
		}
	}
}

func TestBadLogLevel(t *testing.T) {
	defer goroutinechecker.New(t)()

	assert.Panics(t, func() {
		log.NewLogger(nil, log.Level(20))
	})
}
