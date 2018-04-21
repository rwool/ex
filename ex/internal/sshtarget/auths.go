package sshtarget

import "golang.org/x/crypto/ssh"

// Authorizer is a method of authorizing with an SSH server.
type Authorizer interface {
	GetAuthMethod() ssh.AuthMethod
}

// PasswordAuth is a password authentication.
type PasswordAuth struct {
	username string
	ssh.AuthMethod
}

// GetAuthMethod returns the underlying authentication method.
func (pa PasswordAuth) GetAuthMethod() ssh.AuthMethod { return pa.AuthMethod }

// NewPasswordAuth uses password authentication for connecting to an SSH server.
func NewPasswordAuth(password string) PasswordAuth {
	return PasswordAuth{AuthMethod: ssh.Password(password)}
}
