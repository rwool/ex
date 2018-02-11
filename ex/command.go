package ex

import (
	"context"
	"errors"
	"io"
)

// ErrNotRunning indicates a failure due to the command not running.
var ErrNotRunning = errors.New("command not running")

// ErrSignalUnsupported indicates that an attempt to use an unsupported
// signal was made.
var ErrSignalUnsupported = errors.New("unsupported signal")

// Command represents the execution of a command.
//
// Commands are typically created through the usage of Targets.
//
// This interface only represents the basic set of features that are available
// for a command.
// To check for additional functionality, try type asserting to the other
// command* types.
type Command interface {
	// LogEvent logs an arbitrary event.
	LogEvent(eventType string, details interface{})

	// SetInput sets the input of a the command.
	SetInput(stdIn io.Reader)

	// setOutput sets the passthrough output for the command. This is only
	// needed if there is some need to interact with the output of a command as
	// it is being written. All output is by default recorded.
	SetOutput(stdOut, stdErr io.Writer)

	// SetEnv sets the environment variables for the command.
	SetEnv(vars map[string]string)

	// Run runs the command and waits for it to finish.
	Run(ctx context.Context) (*Recorder, error)

	// Start begins execution of the command, but does not wait for it to
	// finish.
	Start(ctx context.Context) (*Recorder, error)

	// Wait waits for a command to finish that was previously started with
	// Start.
	Wait() error
}

// WindowChanger wraps the functions to set/change the window dimensions.
type WindowChanger interface {
	// SetWindowChange sets a channel that can be received from to get terminal
	// window dimension updates.
	SetWindowChange(winChC <-chan struct{ Height, Width int })

	// Sets the terminal dimensions.
	//
	// Calling with values <= 0 for either dimension will unset the requested
	// dimensions.
	SetTerm(height, width int)
}

// Signaller wraps the functions to send a signal.
type Signaller interface {
	// Signal sends a signal to the process.
	// If the command is not running, ErrNotRunning will be returned.
	Signal(Signal) error
}

// CommandWindowChanger is a command that supports changing the window
// dimensions.
type CommandWindowChanger interface {
	// Command is the command.
	Command

	// WindowChanger is capable of changing and setting the window dimensions.
	WindowChanger
}

// CommandSignaller is a command that supports sending signals to it.
type CommandSignaller interface {
	// Command is the command.
	Command

	// Signaller is capable of sending signals.
	Signaller
}

// CommandSignalWinCher is a command that support changing and setting the
// window, as well as sending signals.
type CommandSignalWinCher interface {
	// Command is the command.
	Command

	// WindowChanger is capable of changing and setting the window dimensions.
	WindowChanger

	// Signaller is capable of sending signals.
	Signaller
}
