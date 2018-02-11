package session

import "golang.org/x/crypto/ssh"

// Authorizer is a method of authorizing with an SSH server.
type Authorizer interface {
	getAuthMethod() ssh.AuthMethod
}

type passwordAuth struct {
	username string
	ssh.AuthMethod
}

func (pa passwordAuth) getAuthMethod() ssh.AuthMethod { return pa }

// PasswordAuth uses password authentication for connecting to an SSH server.
func PasswordAuth(password string) Authorizer {
	return passwordAuth{AuthMethod: ssh.Password(password)}
}
