// +build integration

package integration

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rwool/ex/ex/session"

	"github.com/rwool/ex/ex"
	"github.com/rwool/ex/log"
	"github.com/rwool/ex/test/helpers/testlogger"
	"github.com/rwool/ex/test/integration/sshserver"
	"github.com/stretchr/testify/require"
)

type testObjects struct {
	Server sshserver.SSHServer
	Logger log.Logger
	LogBuf *testlogger.Buffer
	Ex     *ex.Ex
	Target ex.Target
}

// setupLoggerAndClient sets up an logger, SSH server, and sets up a target with
// Ex.
func setupLoggerAndClient(tb testing.TB, level log.Level,
	serverType sshserver.ServerType, authorizer session.Authorizer, hkc session.HostKeyCallback) *testObjects {
	logger, logBuf := testlogger.NewTestLogger(tb, level)

	server, err := sshserver.GetSSHServer(serverType, logger)
	require.NoError(tb, err, "failed to get SSH server")

	exBuf := &bytes.Buffer{}
	e := ex.New(logger, exBuf, exBuf)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	target, err := e.NewSSHTarget(ctx, &ex.SSHTargetConfig{
		Name: "test",
		Host: server.Host(),
		Port: server.Port(),
		User: "test",
		Auths: []session.Authorizer{
			authorizer,
		},
		HostKeyCallback: hkc,
	})
	require.NoError(tb, err, "failed to create SSH target")
	require.NotNil(tb, target, "nil target returned")

	return &testObjects{
		Server: server,
		Logger: logger,
		LogBuf: logBuf,
		Ex:     e,
		Target: target,
	}
}
