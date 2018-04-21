package ex

import "github.com/rwool/ex/ex/internal/sshtarget"

// SSHAuthorizer is an SSH authorization method.
type SSHAuthorizer interface {
	sshtarget.Authorizer
	// To disallow other implementations.
	private()
}

func authConvert(a []SSHAuthorizer) []sshtarget.Authorizer {
	t := make([]sshtarget.Authorizer, len(a))
	for i := range a {
		t[i] = a[i]
	}
	return t
}

type sshAuthorizer struct {
	sshtarget.Authorizer
}

func (sshAuthorizer) private() {}

func adaptAuth(auth sshtarget.Authorizer) sshAuthorizer {
	return sshAuthorizer{
		Authorizer: auth,
	}
}

// NewSSHPasswordAuth creates a new password authorizer for SSH targets.
func NewSSHPasswordAuth(password string) SSHAuthorizer {
	return adaptAuth(sshtarget.NewPasswordAuth(password))
}
