// +build integration

package sshserver

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	ssh2 "golang.org/x/crypto/ssh"

	"github.com/gliderlabs/ssh"
	"github.com/pkg/errors"
)

// GliderLabsSSH is an instance of an in-process Glider Labs SSH server.
type GliderLabsSSH struct {
	l        net.Listener
	userPass map[string]string
	userKey  map[string]ssh.Signer
}

// Host returns the hostname of the SSH server.
func (s *GliderLabsSSH) Host() string {
	gURL, err := url.Parse(fmt.Sprintf("ssh://%s", s.l.Addr().String()))
	if err != nil {
		panic(err)
	}
	return gURL.Hostname()
}

// Port returns the port from which the server can be accessed.
func (s *GliderLabsSSH) Port() uint16 {
	gURL, err := url.Parse(fmt.Sprintf("ssh://%s", s.l.Addr().String()))
	if err != nil {
		panic(err)
	}
	p, err := strconv.Atoi(gURL.Port())
	if err != nil {
		panic(err)
	}
	return uint16(p)
}

// Info returns additional information about the server.
func (s *GliderLabsSSH) Info() *ServerInfo {
	u, err := url.Parse(fmt.Sprintf("ssh://%s", s.l.Addr().String()))
	if err != nil {
		panic(err)
	}

	return &ServerInfo{
		URL:                u,
		AllowedAuths:       []AuthMethod{AuthPassword, AuthPublicKey},
		MaxSessionsPerConn: -1,
		UserPass:           s.userPass,
		UserKey:            s.userKey,
	}
}

func (s *GliderLabsSSH) passwordAuth(user, password string) bool {
	if _, ok := s.userPass[user]; ok {
		if s.userPass[user] == password {
			return true
		}
	}
	return false
}

func (s *GliderLabsSSH) addSSHAuthorizedKey(user string) error {
	const errMsg = "unable to add SSH authorized key"

	// Generate "real" private key.
	private, err := genRSA()
	if err != nil {
		return errors.Wrap(err, errMsg)
	}
	privKey, err := ssh2.NewSignerFromSigner(private)
	if err != nil {
		return errors.Wrap(err, errMsg)
	}
	s.userKey[user] = privKey

	return nil
}

func (s *GliderLabsSSH) keyAuth(user string, key ssh.PublicKey) bool {
	if v, ok := s.userKey[user]; ok {
		return ssh.KeysEqual(v.PublicKey(), key)
	}
	return false
}

// NewGliderLabs creates a new in-process Glider Labs SSH server.
func NewGliderLabs() (*GliderLabsSSH, error) {
	l, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return nil, errors.Wrap(err, "unable to create listener")
	}

	ssh.Handle(func(s ssh.Session) {
		rgx := regexp.MustCompile(`sleep (?:(\d+\.\d+)|(\d+$))`)
		matches := rgx.FindAllStringSubmatch(strings.Join(s.Command(), " "), -1)

		if reflect.DeepEqual(s.Command(), []string{"whoami"}) {
			s.Write(append([]byte(s.User()), '\n'))
			s.Exit(0)
		} else if reflect.DeepEqual(s.Command(), []string{"ls", "-a"}) {
			s.Write([]byte(`.
..
.bash_logout
.bashrc
.cache
.profile
`))
			s.Exit(0)
		} else if matches != nil {
			if len(matches[0][1]) > 0 {
				f, err := strconv.ParseFloat(matches[0][1], 64)
				if err != nil {
					fmt.Fprintf(s, "Unable to parse sleep duration: %+v", err)
				}
				time.Sleep(time.Duration(f * float64(time.Second)))
				s.Exit(0)
			} else {
				i, err := strconv.ParseUint(matches[0][2], 10, 32)
				if err != nil {
					fmt.Fprintf(s, "Unable to parse sleep duration: %+v", err)
				}
				time.Sleep(time.Duration(i) * time.Second)
				s.Exit(0)
			}
		}
	})

	s := &GliderLabsSSH{
		l: l,
		userPass: map[string]string{
			"test": "password123",
			"user": "anotherPassword",
			"root": "toor",
		},
		userKey: make(map[string]ssh.Signer),
	}
	for k := range s.userPass {
		s.addSSHAuthorizedKey(k)
	}

	errC := make(chan error, 1)
	go func() {
		errC <- ssh.Serve(l, ssh.DefaultHandler, ssh.PasswordAuth(s.passwordAuth), ssh.PublicKeyAuth(s.keyAuth))
	}()

	// Wait to see if the server can start up successfully.
	select {
	case err = <-errC:
		return nil, errors.Wrap(err, "unable to start Glider Labs SSH server")
	case <-time.After(1 * time.Second):
		return s, nil
	}
}
