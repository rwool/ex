// Package log abstracts the underlying logging implementation.
//
// Separate package from the main Ex code to prevent an import cycle.
package log

import (
	"io"

	"github.com/Sirupsen/logrus"
)

// Level is the minimum log level that will be processed by the logger.
type Level uint8

// Log levels.
const (
	Debug Level = iota + 1 // + 1 to make 0 log level panic.
	Warn
	Error
)

// Logger logs messages with different log levels.
type Logger interface {
	Debug(...interface{})
	Debugf(string, ...interface{})
	Warn(...interface{})
	Warnf(string, ...interface{})
	Error(...interface{})
	Errorf(string, ...interface{})
}

type logger struct {
	*logrus.Logger
}

// NewLogger creates a logger that outputs to the given writer with the minimum
// log level.
func NewLogger(output io.Writer, minLevel Level) Logger {
	l := logger{
		Logger: logrus.New(),
	}

	l.Formatter = &logrus.TextFormatter{}

	l.Out = output
	switch minLevel {
	case Debug:
		l.Level = logrus.DebugLevel
	case Warn:
		l.Level = logrus.WarnLevel
	case Error:
		l.Level = logrus.ErrorLevel
	default:
		panic("invalid level given")
	}

	return l
}
