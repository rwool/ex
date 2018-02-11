// +build integration

package sshserver

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rwool/ex/log"

	"github.com/pkg/errors"
)

// OpenSSHDocker is an OpenSSH server running inside of a Ubuntu Docker container.
type OpenSSHDocker struct {
	ServerInfo
}

// Info gets the information for the Docker SSH container.
func (osd *OpenSSHDocker) Info() *ServerInfo {
	return &osd.ServerInfo
}

// Host returns the host, without port, for the SSH server.
func (osd *OpenSSHDocker) Host() string {
	return osd.URL.Hostname()
}

// Port returns the port for the SSH server.
func (osd *OpenSSHDocker) Port() uint16 {
	portU64, err := strconv.Atoi(osd.URL.Port())
	if err != nil {
		panic(err)
	}

	return uint16(portU64)
}

var args = struct {
	DockerCommand string
}{}

func init() {
	flag.StringVar(&args.DockerCommand, "docker-command", "docker",
		"command to use to invoke docker")
}

func dockerArgs(hostPort int, imageName string, duration time.Duration) []string {
	const cmdFmt = "service ssh start && printf '\nEx Container is Running\n' && exec sleep %d"
	tmp := fmt.Sprintf(`
run
--rm
--init
-p %d:22
%s
sh -c
`, hostPort, imageName)

	cmdStr := fmt.Sprintf(cmdFmt, int(duration.Seconds()))

	return append(strings.Fields(tmp), cmdStr)
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	if err == nil {
		return true
	}

	execErr := err.(*exec.Error)
	if execErr.Err == exec.ErrNotFound {
		return false
	}
	panic(err)
}

func canUseDocker() error {
	// TODO: Handle remote machines too.
	errOut := func(e error) error {
		return errors.Wrap(e, "unable to check Docker usability")
	}

	if runtime.GOOS == "darwin" {
		// Assuming that Docker installs on macOS are working.
		return nil
	}

	curUser, err := user.Current()
	if err != nil {
		return errOut(err)
	}

	if curUser.Uid == "0" {
		// Is root.
		return nil
	}

	gids, err := curUser.GroupIds()
	if err != nil {
		return errOut(err)
	}
	for _, gid := range gids {
		group, err := user.LookupGroupId(gid)
		if err != nil {
			return errOut(err)
		}

		if group.Name == "docker" {
			return nil
		}
	}

	return errors.New("user not in docker group")
}

func attemptDockerStart(logger log.Logger, fn func(**exec.Cmd, *context.Context, *context.CancelFunc), er *error) <-chan struct{} {
	finishC := make(chan struct{})
	go func() {
		var tmpErr error
		defer func() { *er = tmpErr }()

		var cmd *exec.Cmd
		var ctx context.Context
		var canFn context.CancelFunc
		var outBuf *bytes.Buffer
		for i := 0; i < 5; i++ {
			fn(&cmd, &ctx, &canFn)
			outBuf = &bytes.Buffer{}
			pR, pW := io.Pipe()

			go func() {
				scanner := bufio.NewScanner(pR)
				for scanner.Scan() {
					if scanner.Text() == "Ex Container is Running" {
						close(finishC)
						_, err := io.Copy(ioutil.Discard, pR)
						if err != nil {
							logger.Errorf("Error copying out from command: %v", err)
							tmpErr = errors.Wrap(err, "failed to finish copying out from command")
							canFn()
						}
					}
				}
				if err := scanner.Err(); err != nil {
					logger.Errorf("Error scanning out from command: %v", err)
					tmpErr = errors.Wrap(err, "failed to scan for starting string")
					canFn()
				}
			}()

			w := io.MultiWriter(outBuf, pW)
			cmd.Stdout = w
			cmd.Stderr = w
			tmpErr = errors.Wrap(cmd.Run(), "error attempting to start Docker")
			if tmpErr == nil {
				logger.Errorf("Error running command: %+v", tmpErr)
				canFn()
				break
			}
		}
		if tmpErr != nil {
			if outBuf.Len() > 0 {
				tmpErr = errors.Wrapf(tmpErr, "error attempting to start Docker (output: %s)", outBuf.String())
			}
			close(finishC)
		}
	}()

	return finishC
}

// runDocker runs an OpenSSH server in a Docker container.
//
// The command blocks until either the container successfully starts or all
// attempts to start the container have failed.
func runDocker(ctx context.Context, duration time.Duration, logger log.Logger) (hostPort int, er error) {
	errOut := func(e error) (int, error) {
		return 0, errors.Wrap(e, "unable to run docker command")
	}

	var dArgs []string
	var port int
	dFn := func() {
		port = rand.Int()%500 + 34000
		dArgs = dockerArgs(
			port, "ex-integration", duration,
		)
	}

	var fn func(**exec.Cmd, *context.Context, *context.CancelFunc)

	dc := strings.Fields(args.DockerCommand)
	switch len(dc) {
	case 0:
		return errOut(errors.New("invalid Docker command given: empty"))
	case 1:
		fn = func(c **exec.Cmd, cmdCtx *context.Context, cmdCanceller *context.CancelFunc) {
			dFn()
			*cmdCtx, *cmdCanceller = context.WithCancel(ctx)
			*c = exec.CommandContext(*cmdCtx, args.DockerCommand, dArgs...)
		}
	default:
		fn = func(c **exec.Cmd, cmdCtx *context.Context, cmdCanceller *context.CancelFunc) {
			dFn()
			*cmdCtx, *cmdCanceller = context.WithCancel(ctx)
			*c = exec.CommandContext(*cmdCtx, dc[0], append(dc[1:], dArgs...)...)
		}
	}

	var err error
	<-attemptDockerStart(logger, fn, &err)
	if err != nil {
		return 0, errors.Wrap(err, "unable to run Docker")
	}
	return port, nil
}

var (
	dockerOnce      sync.Once
	dockerInfoReady = make(chan struct{})
	dockerInfo      *ServerInfo
	dockerErr       error
)

func startDocker(ctx context.Context, logger log.Logger) (info *ServerInfo, e error) {
	const eStr = "unable to start Docker"

	err := ensureIntImageExists(ctx)
	if err != nil {
		return nil, errors.Wrap(err, eStr)
	}

	if err = canUseDocker(); err != nil {
		return nil, errors.Wrap(err, eStr)
	}

	port, err := runDocker(context.Background(), 30*time.Second, logger)
	if err != nil {
		return nil, errors.Wrap(err, eStr)
	}

	dURL, err := url.Parse(fmt.Sprintf("ssh://127.0.0.1:%d", port))
	if err != nil {
		return nil, errors.Wrap(err, eStr)
	}

	auths := []AuthMethod{
		AuthPassword,
		AuthKeyboardInteractive,
		AuthPublicKey,
	}

	maxSessionsPerConn := 10

	return &ServerInfo{
		URL:                dURL,
		AllowedAuths:       auths,
		MaxSessionsPerConn: maxSessionsPerConn,
	}, nil
}

// NewOpenSSH starts an OpenSSH server in a Docker container.
func NewOpenSSH(logger log.Logger) (server *OpenSSHDocker, e error) {
	dockerOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		dockerInfo, dockerErr = startDocker(ctx, logger)
		close(dockerInfoReady)
	})

	<-dockerInfoReady
	if dockerErr != nil {
		return nil, dockerErr
	}
	return &OpenSSHDocker{
		ServerInfo: *dockerInfo,
	}, nil
}
