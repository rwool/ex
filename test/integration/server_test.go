// +build integration

package integration

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rwool/ex/ex"

	"github.com/rwool/ex/ex/escape"

	"github.com/rwool/ex/test/helpers/blockingreader"
	"github.com/rwool/ex/test/helpers/cartesian"

	"github.com/stretchr/testify/require"

	"github.com/pkg/errors"

	"github.com/rwool/ex/test/integration/sshserver"

	"github.com/stretchr/testify/assert"

	"github.com/rwool/ex/log"
)

func TestWhoami(t *testing.T) {
	serverTypes := []interface{}{sshserver.OpenSSH, sshserver.GliderLabs}
	c := cartesian.New(serverTypes)
	for c.Next() {
		sType := c.Slice()[0].(sshserver.ServerType)
		t.Run(sType.String(), func(t2 *testing.T) {
			testWhoami(t2, sType)
		})
	}
}

func testWhoami(t *testing.T, sType sshserver.ServerType) {
	to := setupLoggerAndClient(t, log.Warn,
		sType, ex.NewSSHPasswordAuth("password123"),
		ex.SSHInsecureIgnoreHostKey())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rec, err := to.Target.Command("whoami").Run(ctx)
	assert.NoError(t, err)
	buf := &bytes.Buffer{}
	err = rec.Replay(buf, buf, 0)
	assert.NoError(t, err)
	assert.Equal(t, "test\n", buf.String())
	assert.Empty(t, string(to.LogBuf.Bytes()))
}

func runCommandWithExpectedOutput(t *testing.T, command string, buf *bytes.Buffer, expectedOutput string) error {
	t.Helper()

	to := setupLoggerAndClient(t, log.Warn,
		sshserver.OpenSSH, ex.NewSSHPasswordAuth("password123"),
		ex.SSHInsecureIgnoreHostKey())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := to.Target.Command(command)
	rec, err := cmd.Run(ctx)
	if err != nil {
		return errors.Wrap(err, "failure to run command")
	}

	recErr := rec.Replay(buf, buf, 0)
	if recErr != nil {
		return errors.Wrap(recErr, "command run recording replay failure")
	}
	if err != nil {
		return errors.Wrapf(err, "failed to run command: (Output: %q)", buf.String())
	}

	if buf.String() != expectedOutput {
		return errors.New("output received does not match expected output")
	}

	return nil
}

func TestMaxSessions(t *testing.T) {
	serverTypes := []interface{}{sshserver.OpenSSH, sshserver.GliderLabs}
	c := cartesian.New(serverTypes)
	for c.Next() {
		sType := c.Slice()[0].(sshserver.ServerType)
		t.Run(sType.String(), func(t2 *testing.T) {
			testMaxSessions(t2, sType)
		})
	}
}

func testMaxSessions(t *testing.T, sType sshserver.ServerType) {
	tcs := []struct {
		Name    string
		Command string
		Output  string
	}{
		{
			Name:    "Whoami",
			Command: "whoami",
			Output:  "test\n",
		},
		{
			Name:    "Ls One Argument",
			Command: "ls -a",
			Output: `.
..
.bash_logout
.bashrc
.cache
.profile
`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t2 *testing.T) {

			to := setupLoggerAndClient(t2, log.Warn,
				sType, ex.NewSSHPasswordAuth("password123"),
				ex.SSHInsecureIgnoreHostKey())

			var numGR int
			if mspc := to.Server.Info().MaxSessionsPerConn; mspc == -1 {
				numGR = 10
			} else {
				numGR = mspc
			}

			var readyWG sync.WaitGroup
			readyWG.Add(numGR)
			startC := make(chan struct{})
			errC := make(chan error)
			for i := 0; i < numGR; i++ {
				go func() {
					var buf bytes.Buffer
					readyWG.Done()
					<-startC
					errC <- runCommandWithExpectedOutput(t, tc.Command, &buf, tc.Output)
				}()
			}

			var err error
			readyWG.Wait()
			close(startC)
			var firstError error
			for i := 0; i < numGR; i++ {
				select {
				case err = <-errC:
				case <-time.After(20 * time.Second):
					t2.Fatal("timeout waiting for command to finish")
				}

				if firstError == nil {
					firstError = err
				}
			}
			require.NoError(t2, firstError)

			assert.Empty(t2, to.LogBuf.Bytes())
		})
	}
}

func TestEscapes(t *testing.T) {
	serverTypes := []interface{}{sshserver.OpenSSH, sshserver.GliderLabs}
	c := cartesian.New(serverTypes)
	for c.Next() {
		sType := c.Slice()[0].(sshserver.ServerType)
		t.Run(sType.String(), func(t2 *testing.T) {
			testEscapes(t2, sType)
		})
	}
}

func testEscapes(t *testing.T, sType sshserver.ServerType) {
	tcs := []struct {
		Name           string
		Command        string
		EscapeSequence []byte
		EscapeEvents   []ex.SpecialEvent
		After          time.Duration
		Output         string
	}{
		{
			Name:           "No Escape",
			Command:        "sleep 3",
			EscapeSequence: []byte("a bc d 123     4"),
			EscapeEvents:   []ex.SpecialEvent{},
			After:          0,
			Output:         "",
		},
		{
			Name:           "Basic Escape",
			Command:        "sleep 7",
			EscapeSequence: []byte("\n~."),
			EscapeEvents: []ex.SpecialEvent{
				{
					EventType: ex.EscapeEvent,
					Details:   []byte("\n~."),
				},
			},
			After:  5 * time.Second,
			Output: "",
		},
		{
			Name:           "Escape Prefix",
			Command:        "sleep 4",
			EscapeSequence: []byte("\n~"),
			EscapeEvents:   []ex.SpecialEvent{},
			After:          2 * time.Second,
			Output:         "",
		},
		{
			Name:           "Escape Prefix Followed By Basic Escape",
			Command:        "sleep 4",
			EscapeSequence: []byte("\n~\n~."),
			EscapeEvents: []ex.SpecialEvent{
				{
					EventType: ex.EscapeEvent,
					Details:   []byte("\n~."),
				},
			},
			After:  2 * time.Second,
			Output: "",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t2 *testing.T) {
			testStart := time.Now()
			expectedTestEnd := testStart.Add(5*time.Second + tc.After)

			to := setupLoggerAndClient(t2, log.Warn,
				sType, ex.NewSSHPasswordAuth("password123"),
				ex.SSHInsecureIgnoreHostKey())

			r := bytes.NewReader(tc.EscapeSequence)
			br := blockingreader.NewBlockingReader(tc.After, r)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd := to.Target.Command(tc.Command)
			if v, ok := cmd.(ex.WindowChanger); ok {
				v.SetTerm(24, 80)
			} else {
				assert.FailNow(t2, "unable to set window dimensions with command")
			}
			es := escape.NewReader(br, []byte("\n~."), func() {
				cmd.LogEvent(ex.EscapeEvent, []byte("\n~."))
				cancel()
			})
			cmd.SetInput(es)
			rec, err := cmd.Run(ctx)
			if err != nil {
				br.Cancel()
				require.NoError(t2, err)
			}

			se := rec.GetSpecialEvents()
			require.Len(t2, se, len(tc.EscapeEvents), "wrong number of escape events recorded")
			for i, event := range tc.EscapeEvents {
				assert.Equal(t2, event.EventType, se[i].EventType, "unexpected event type")
				assert.Equal(t2, event.Details, se[i].Details, "wrong event details")
				assert.True(t2, se[i].Timestamp.After(testStart), "timestamp before test started")
				assert.True(t2, se[i].Timestamp.Before(expectedTestEnd), "timestamp after expected test end")
			}
		})
	}
}
