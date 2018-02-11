// +build integration

package sshserver

import (
	"net/url"

	"github.com/rwool/ex/log"

	"github.com/gliderlabs/ssh"
)

// OpenSSHDocker wraps the Info method for getting information about a generic SSH
// server.
type SSHServer interface {
	Host() string
	Port() uint16
	Info() *ServerInfo
}

// ServerType is a type of SSH server that can be connected to.
type ServerType uint8

// List of the SSH server types that can be connected to.
const (
	OpenSSH ServerType = iota
	GliderLabs
)

// GetSSHServer gets an SSH server to connect to.
func GetSSHServer(st ServerType, logger log.Logger) (SSHServer, error) {
	switch st {
	case OpenSSH:
		return NewOpenSSH(logger)
	case GliderLabs:
		return NewGliderLabs()
	default:
		panic("unknown SSH server type")
	}
}

// AuthMethod is an authentication method supported by an SSH server.
type AuthMethod uint8

// List of authentication methods an SSH server can support.
const (
	AuthPassword AuthMethod = iota
	AuthKeyboardInteractive
	AuthPublicKey
)

// ServerInfo contains information about an SSH server.
type ServerInfo struct {
	// URL is where the SSH server can be accessed.
	// The schema is "ssh://".
	URL *url.URL
	// MaxSessionsPerConn is the maximum number of sessions that can be open
	// per network connection. Does not include forwarding.
	// A value of -1 indicates that there is no limit on the number of
	// connections.
	MaxSessionsPerConn int
	// AllowedAuths is the allowed authentications methods.
	AllowedAuths []AuthMethod
	// UserPass is a mapping of accepted usernames to password.
	UserPass map[string]string
	// UserKey is a mapping of accepted usernames to private keys.
	// Public keys can be generated from these private keys.
	UserKey map[string]ssh.Signer
}
