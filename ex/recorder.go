package ex

import (
	"bytes"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	errors2 "errors"

	"github.com/fatih/color"
	"github.com/pkg/errors"

	"github.com/kballard/go-shellquote"
)

// ErrInvalidMultiplier indicates an negative value was given for the replay
// speed.
var ErrInvalidMultiplier = errors2.New("invalid speed multiplier")

// Events that can be recorded.
const (
	EscapeEvent = "Escape"
)

// SpecialEvent contains the metadata for an event.
type SpecialEvent struct {
	EventType string
	Timestamp time.Time
	Details   interface{}
}

type outputType uint8

const (
	stdout outputType = iota
	stderr
)

// eventBuffer handles a single output stream.
type eventBuffer struct {
	bytes.Buffer
	startTime time.Time
	outBuffer *[]outEvent
	bufMu     *sync.Mutex
	oType     outputType
	scratch   [64]byte

	passthrough io.Writer
}

// ReadFrom reads from the given reader (usually stdin or stdout) and writes it
// out to the passthrough writer, if one is set.
// Reading is done until the given io.Reader returns an io.EOF or some other
// error.
func (eb *eventBuffer) ReadFrom(r io.Reader) (int64, error) {
	var totalRead int64
	for {
		read, err := r.Read(eb.scratch[:])
		if err != nil {
			if err == io.EOF {
				return totalRead + int64(read), nil
			}
		}
		totalRead += int64(read)
		eb.Write(eb.scratch[:read])
	}
}

// Write handles the recording of a single write from an output stream.
func (eb *eventBuffer) Write(p []byte) (int, error) {
	startOffset := eb.Len()
	written, _ := eb.Buffer.Write(p)
	bufBytes := eb.Buffer.Bytes()

	eb.bufMu.Lock()
	*eb.outBuffer = append(*eb.outBuffer, outEvent{
		timeOffset: time.Since(eb.startTime),
		source:     eb.oType,
		data:       bufBytes[startOffset : startOffset+written],
	})
	eb.bufMu.Unlock()

	// TODO: Have option to log/store error but not report it here.
	if eb.passthrough != nil {
		written, err := eb.passthrough.Write(p)
		if err != nil {
			return written, err
		}
		if written < len(p) {
			return written, errors.Errorf("bad writer: only %d bytes written, but expected %d", written, len(p))
		}
	}

	return written, nil
}

type outEvent struct {
	timeOffset time.Duration
	source     outputType
	data       []byte
}

// Recorder handles the recording of data for a command.
type Recorder struct {
	cmd  string
	args []string

	out eventBuffer
	err eventBuffer

	events   []SpecialEvent
	eventsMu sync.Mutex

	recordingStart time.Time
	entries        []outEvent
	writeMu        sync.Mutex
	stateMu        sync.Mutex

	eventNotifier  chan struct{}
	readLiveEvents int
}

// command outputs the escaped command string that is suitable for use with SSH.
func (r *Recorder) command() string {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	tmp := []string{r.cmd}
	tmp = append(tmp, r.args...)
	return strings.Join(tmp, " ")
}

// commandEscaped returns the escaped version of the command for using in
// scripts.
func (r *Recorder) commandEscaped() string {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	tmp := []string{r.cmd}
	tmp = append(tmp, r.args...)
	return shellquote.Join(tmp...)
}

// startTiming sets the beginning offset off of which future timestamps are
// based.
//
// Subsequent calls to this function will have no effect.
func (r *Recorder) startTiming() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	if r.recordingStart.IsZero() {
		r.recordingStart = time.Now()
		r.out.startTime = r.recordingStart
		r.err.startTime = r.recordingStart
	}
}

// NewRecorder returns a new Recorder.
func NewRecorder() *Recorder {
	r := &Recorder{}

	return r
}

// setCommand sets the command that is to be executed.
func (r *Recorder) setCommand(cmd string, args ...string) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	r.cmd = cmd
	r.args = args
}

// setOutput sets the outputs pointed to to the types that the recorder will use
// to record the data.
//
// This is for setting commands to output to the recorder.
func (r *Recorder) setOutput(out *io.Writer, err *io.Writer) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	if out == nil {
		panic("nil out pointer")
	}
	if err == nil {
		panic("nil err pointer")
	}

	r.out = eventBuffer{
		outBuffer: &r.entries,
		bufMu:     &r.writeMu,
		oType:     stdout,
	}
	*out = &r.out
	r.err = eventBuffer{
		outBuffer: &r.entries,
		bufMu:     &r.writeMu,
		oType:     stderr,
	}
	*err = &r.err
}

// setPassthrough sets the pair of outputs that will be written to as data comes
// in to the recorder.
func (r *Recorder) setPassthrough(out io.Writer, err io.Writer) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	if r.out.bufMu == nil || r.err.bufMu == nil {
		panic("output not set yet")
	}

	r.out.passthrough = out
	r.err.passthrough = err
}

// GetSpecialEvents gets all of the special events that have been recorded.
func (r *Recorder) GetSpecialEvents() []SpecialEvent {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()

	return r.events
}

// AddSpecialEvent records a new special event.
func (r *Recorder) AddSpecialEvent(eventType string, details interface{}) {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()

	r.events = append(r.events, SpecialEvent{
		EventType: eventType,
		Timestamp: time.Now(),
		Details:   details,
	})
}

// Replay replays the given output to the given streams.
//
// If the speed multiplier is 0, then the output will happen as fast as
// possible. Speed multipliers less than 0 will return ErrInvalidMultiplier.
func (r *Recorder) Replay(out io.Writer, err io.Writer, speedMultiplier float64) error {
	if out == nil && err == nil {
		panic("only given nil writers")
	}
	if out != nil && err == nil {
		err = out
	} else if out == nil && err != nil {
		out = err
	}

	sm := speedMultiplier
	if sm < 0 || math.IsInf(sm, 0) || math.IsNaN(sm) {
		return ErrInvalidMultiplier
	}

	replayStart := time.Now()
	var elapsed time.Duration
	for i := range r.entries {
		entry := &r.entries[i]

		if sm == 0 {
			// Don't sleep.
		} else if sm == 1 {
			runTime := r.recordingStart.Add(entry.timeOffset)
			sleepDuration := runTime.Sub(r.recordingStart) - elapsed
			time.Sleep(sleepDuration)
		} else {
			offset := time.Duration(float64(entry.timeOffset) / sm)
			runTime := r.recordingStart.Add(offset)
			sleepDuration := runTime.Sub(r.recordingStart) - elapsed
			time.Sleep(sleepDuration)
		}
		elapsed = time.Since(replayStart)

		fn := func(w io.Writer, colorize bool) error {
			offset := 0
			for {
				var written int
				var err error
				if colorize {
					written, err = color.New(color.FgGreen).Fprint(w, string(entry.data[offset:]))
				} else {
					written, err = w.Write(entry.data[offset:])
				}
				if err != nil {
					return err
				}
				if offset+written < len(entry.data) {
					offset += written
				} else {
					break
				}
			}
			return nil
		}
		if entry.source == stdout {
			if err := fn(out, false); err != nil {
				return err
			}
		} else {
			if err := fn(err, true); err != nil {
				return err
			}
		}
	}
	return nil
}
