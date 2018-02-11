// +build integration

package sshserver

import (
	"crypto/rand"
	"crypto/rsa"

	"github.com/pkg/errors"
)

// genRSA generates an RSA private key.
func genRSA() (*rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 128)
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate RSA private key")
	}

	return priv, nil
}
