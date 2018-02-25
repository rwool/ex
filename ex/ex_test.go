package ex_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"github.com/rwool/ex/ex/session"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/rwool/ex/test/helpers/testlogger"

	"github.com/rwool/ex/ex"
	"github.com/stretchr/testify/require"

	"github.com/rwool/ex/log"
)

func TestEx(t *testing.T) {
	defer goroutinechecker.New(t)()

	logger, logBuf := testlogger.NewTestLogger(t, log.Warn)
	dialer, stopServer := NewSSHServer(logger)
	defer func() {
		stopServer()
		time.Sleep(50 * time.Millisecond)
	}()
	if v, ok := dialer.(io.Closer); ok {
		defer v.Close()
	}

	e := ex.New(logger, nil, nil)
	e.SetDialer(dialer)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	target, err := e.NewSSHTarget(ctx, &ex.SSHTargetConfig{
		Name: "Server 1",
		Host: "127.0.0.1",
		Port: 22,
		User: "test",
		Auths: []session.Authorizer{
			session.PasswordAuth("Password123"),
		},
	})
	require.NoError(t, err, "error creating target")

	cmd := target.Command("whoami")
	rec, err := cmd.Run(ctx)
	require.NoError(t, err, "error running whoami")

	var stdout, stderr bytes.Buffer

	err = rec.Replay(&stdout, &stderr, 0)
	require.NoError(t, err, "error replaying recording")

	assert.Equal(t, "test\n", stdout.String(), "unexpected stdout output")
	assert.Empty(t, stderr.String(), "unexpected data in stderr")

	require.NoError(t, e.Close(), "unexpected error closing Ex")

	assert.Empty(t, logBuf.String(), "unexpected log output")
}

func TestExFailedLogin(t *testing.T) {
	defer goroutinechecker.New(t)()

	logger, logBuf := testlogger.NewTestLogger(t, log.Warn)
	dialer, stopServer := NewSSHServer(logger)
	defer func() {
		stopServer()
		time.Sleep(50 * time.Millisecond)
	}()
	if v, ok := dialer.(io.Closer); ok {
		defer v.Close()
	}

	e := ex.New(logger, nil, nil)
	e.SetDialer(dialer)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_, err := e.NewSSHTarget(ctx, &ex.SSHTargetConfig{
		Name: "Server 1",
		Host: "127.0.0.1",
		Port: 22,
		User: "test",
		Auths: []session.Authorizer{
			session.PasswordAuth("wrong"),
		},
	})

	defer func() {
		require.NoError(t, e.Close(), "error closing Ex")
		require.Empty(t, logBuf.String(), "unexpected log output")
	}()
	assert.True(t, strings.Contains(errors.Cause(err).Error(), "ssh: handshake failed"),
		"unexpected error from failing to authenticate")
}
