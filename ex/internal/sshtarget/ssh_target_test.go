package sshtarget

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"
	"github.com/stretchr/testify/require"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"github.com/rwool/ex/ex/internal/recorder"
	"github.com/rwool/ex/log"
	"github.com/rwool/ex/test/helpers/testlogger"
)

func TestConnectionSSH(t *testing.T) {
	defer goroutinechecker.New(t)()

	logger, _ := testlogger.NewTestLogger(t, log.Warn)
	dialer, hostKey, stopServer := NewSSHServer(logger)
	defer func() {
		stopServer()
		time.Sleep(50 * time.Millisecond)
	}()
	if v, ok := dialer.(io.Closer); ok {
		defer v.Close()
	}

	ctx, canceller := context.WithTimeout(context.Background(), 5*time.Second)
	defer canceller()

	validateRecording := func(r *recorder.Recorder, has string) {
		buf := &bytes.Buffer{}
		err := r.Replay(buf, buf, 0)
		assert.NoError(t, err)
		assert.Equal(t, has, buf.String())
	}

	hkOpt := HostKeyValidationOption(FixedHostKey(hostKey))
	c, err := New(ctx, logger, dialer, "127.0.0.1", 22,
		[]Option{hkOpt}, "test",
		[]Authorizer{NewPasswordAuth("Password123")})
	require.NoError(t, err, "error getting SSH target")

	rec, err := c.Command("whoami").Run(ctx)
	assert.NoError(t, err)
	validateRecording(rec, "test\n")

	rec, err = c.Command("doesNotExist").Run(ctx)
	assert.EqualError(t, errors.Cause(err), "Process exited with status 127")
	validateRecording(rec, "-bash: doesNotExist: command not found\n")

	assert.NoError(t, c.Close(), "unexpected error from SSH target close")
}
