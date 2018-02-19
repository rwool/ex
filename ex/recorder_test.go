package ex

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

type ChanWriter struct {
	c chan []byte
}

func (cw *ChanWriter) Write(p []byte) (int, error) {
	cw.c <- p
	return len(p), nil
}

func TestRecorder(t *testing.T) {
	defer goroutinechecker.New(t)()

	rec := NewRecorder()

	var stdoutWriter, stderrWriter io.Writer
	rec.setOutput(&stdoutWriter, &stderrWriter)

	var outBuf, errBuf bytes.Buffer
	rec.setPassthrough(&outBuf, &errBuf)

	rec.startTiming()
	stdoutWriter.Write([]byte("Hello"))

	buf := bytes.Buffer{}

	time.Sleep(1 * time.Second)
	stderrWriter.Write([]byte("There"))

	assert.Equal(t, "Hello", outBuf.String())
	assert.Equal(t, "There", errBuf.String())

	replayStart := time.Now()
	rec.Replay(&buf, &buf, 0)
	assert.Contains(t, buf.String(), "HelloThere", "missing correct output")
	assert.True(t, time.Since(replayStart) < 5*time.Millisecond, "replay not instant")

	byteOutC, byteErrC := make(chan []byte), make(chan []byte)
	cwOut, cwErr := ChanWriter{c: byteOutC}, ChanWriter{c: byteErrC}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		replayStart = time.Now()
		wg.Done()
		rec.Replay(&cwOut, &cwErr, 1)
	}()
	wg.Wait()

	for i := 0; i < 2; i++ {
		select {
		case b := <-byteOutC:
			assert.True(t,
				time.Since(replayStart) < 200*time.Millisecond,
				"stdout replay took too long")
			assert.Equal(t, []byte("Hello"), b)
		case b := <-byteErrC:
			assert.True(t,
				time.Since(replayStart) > (1*time.Second),
				"stderr replay happened early")
			assert.True(t,
				time.Since(replayStart) < (1*time.Second)+(500*time.Millisecond),
				"stderr replay took too long")
			assert.Equal(t, []byte("There"), b)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for replay output")
		}
	}
}

func TestRecorderCommandOutput(t *testing.T) {
	defer goroutinechecker.New(t)()

	tcs := []struct {
		Name              string
		Command           string
		Args              []string
		CommandOut        string
		CommandEscapedOut string
	}{
		{
			Name:              "Single Word",
			Command:           "test",
			Args:              []string{},
			CommandOut:        "test",
			CommandEscapedOut: "test",
		},
		{
			Name:              "Single Word One Simple Arg",
			Command:           "test",
			Args:              []string{"-a"},
			CommandOut:        "test -a",
			CommandEscapedOut: "test -a",
		},
		{
			Name:              "Single Word One Quoted Arg",
			Command:           "test",
			Args:              []string{`"-a"`},
			CommandOut:        `test "-a"`,
			CommandEscapedOut: `test \"-a\"`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t2 *testing.T) {
			defer goroutinechecker.New(t2)()
			rec := NewRecorder()
			rec.setCommand(tc.Command, tc.Args...)
			assert.Equal(t2, tc.CommandOut, rec.command())
			assert.Equal(t2, tc.CommandEscapedOut, rec.commandEscaped())
		})
	}
}

func TestRecorderAddSpecialEvents(t *testing.T) {
	defer goroutinechecker.New(t)()

	rec := NewRecorder()

	rec.startTiming()
	rec.AddSpecialEvent(EscapeEvent, "test")
	time.Sleep(1 * time.Second)
	rec.AddSpecialEvent(EscapeEvent, "word")

	se := rec.GetSpecialEvents()
	require.Len(t, se, 2, "fewer special events than expected")
	ev := se[0]
	assert.Equal(t, EscapeEvent, ev.EventType)
	assert.Equal(t, ev.Details, "test")
	assert.True(t, time.Since(ev.Timestamp) > 1*time.Second,
		"event time before acceptable range")
	assert.True(t, time.Since(ev.Timestamp) < (1*time.Second)+(500*time.Millisecond),
		"event time after acceptable range")
	ev = se[1]
	assert.Equal(t, EscapeEvent, ev.EventType)
	assert.Equal(t, ev.Details, "word")
	assert.True(t, time.Since(ev.Timestamp) < 20*time.Millisecond,
		"event time after acceptable range")
}

func TestRecorderReplayNilWriters(t *testing.T) {
	defer goroutinechecker.New(t)()

	rec := NewRecorder()
	assert.Panics(t, func() {
		rec.Replay(nil, nil, 0)
	})
}
