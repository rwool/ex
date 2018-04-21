package ex

import (
	"io"

	"github.com/rwool/ex/ex/internal/recorder"
)

// SpecialEvent contains the metadata for an event.
type SpecialEvent = recorder.SpecialEvent

// Events that can be recorded.
const (
	EscapeEvent = recorder.EscapeEvent
)

// Recorder wraps the set of methods for interacting with a recording of a
// Target session.
type Recorder interface {
	Replay(out io.Writer, err io.Writer, speedMultipler float64) error
	GetSpecialEvents() []SpecialEvent
}
