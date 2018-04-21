package sshtarget

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const globalKnownHosts = "/etc/ssh/ssh_known_hosts"

type fileOpener func(string) (io.Closer, error)

// getKnownHostPaths checks for known_hosts files in the locations specified by
// sshd(8). These are "~/.ssh/known_hosts" and, optionally,
// /etc/ssh/ssh_known_hosts.
//
// The fileOpener argument exists to make the code not have a hard dependency on
// os.Open.
func getKnownHostPaths(fo fileOpener) ([]string, error) {
	u, err := user.Current()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get user for known_hosts file")
	}
	userKH := filepath.Join(u.HomeDir, ".ssh", "known_hosts")
	userF, userErr := fo(userKH)
	if userErr == nil {
		err = userF.Close()
		if err != nil {
			return nil, errors.Wrap(err, "unable to close user known_hosts")
		}
	}

	globalF, globalErr := fo(globalKnownHosts)
	if globalErr == nil {
		err = globalF.Close()
		if err != nil {
			return nil, errors.Wrap(err, "unable to close global known_hosts")
		}
	}

	if userErr != nil && globalErr != nil {
		// Just wrapping one error here, for simplicity.
		return nil, errors.Wrap(userErr, "unable to find usable known_hosts file")
	} else if userErr != nil {
		return []string{globalKnownHosts}, nil
	} else if globalErr != nil {
		return []string{userKH}, nil
	}
	return []string{globalKnownHosts, userKH}, nil
}

// KnownHostsMarker represents the usage of a marker in a known_hosts file.
type KnownHostsMarker uint8

// Markers available to use with lines in the known_hosts file(s).
const (
	MarkerNone KnownHostsMarker = iota
	MarkerCertAuthority
	MarkerRevoked
)

// String returns the string of the marker, including the leading @ symbol.
func (khm KnownHostsMarker) String() string {
	switch khm {
	case MarkerNone:
		return ""
	case MarkerCertAuthority:
		return mCertAuthority
	case MarkerRevoked:
		return mRevoked
	default:
		panic("unknown marker")
	}
}

const (
	mCertAuthority = "@cert-authority"
	mRevoked       = "@revoked"
)

// AddToKnownHosts adds hosts to the given known_hosts file.
//
// Disabling hashing has the effect of not hashing the hostnames provided to
// this function.
// Not hashing has the effect of making the known_hosts file more readable, but
// at the expense of some security if the known_hosts file is maliciously
// obtained.
//
// In general usage, this function should be called if the HostKeyCallback
// returns an knownhosts.KeyError.
// Note that if the Want field of the KeyError is not empty, then that may be an
// indication of a MITM attack.
func AddToKnownHosts(f io.WriteSeeker, hosts []string, pubKey ssh.PublicKey, disableHashing bool, marker KnownHostsMarker) error {
	addrs := make([]string, len(hosts))
	for i, v := range hosts {
		norm := knownhosts.Normalize(v)
		if disableHashing {
			addrs[i] = norm
		} else {
			addrs[i] = knownhosts.HashHostname(norm)
		}
	}
	newLine := knownhosts.Line(addrs, pubKey)
	_, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return errors.Wrap(err, "unable to add known_hosts line")
	}

	var m string
	switch marker {
	case MarkerNone:
	case MarkerCertAuthority:
		m = mCertAuthority + " "
	case MarkerRevoked:
		m = mRevoked + " "
	}

	_, err = fmt.Fprint(f, m, newLine)
	return errors.Wrap(err, "unable to write new known_hosts line")
}

// DefaultKnownHosts returns the standard known_hosts file paths
// for the known_hosts files that are found.
func DefaultKnownHosts() ([]string, error) {
	return getKnownHostPaths(func(s string) (io.Closer, error) {
		return os.Open(s)
	})
}

// KnownHostsFilesCallback creates a host key callback using the hosts
// specified in the given known_hosts files.
//
// To support adding host keys dynamically, the returned HostKeyCallback may
// be wrapped in a function to support handling the returned errors from the
// callback.
func KnownHostsFilesCallback(knownHostsPaths ...string) (ssh.HostKeyCallback, error) {
	return knownhosts.New(knownHostsPaths...)
}

// IsUnknownHost indicates if the given error was due to an unknown host.
func IsUnknownHost(e error) bool {
	if ke, ok := e.(*knownhosts.KeyError); ok {
		return len(ke.Want) == 0
	}
	return false
}

// IsKeyChange indicates if the given error was due to an unexpected host
// key for an already known host.
//
// If this returns true, then it may be an indication of a
// man-in-the-middle attack.
func IsKeyChange(e error) bool {
	if ke, ok := e.(*knownhosts.KeyError); ok {
		return len(ke.Want) > 0
	}
	return false
}

// IsRevoked indicates if the given error was due to a revoked host key
// being given.
func IsRevoked(e error) bool {
	_, ok := e.(*knownhosts.RevokedError)
	return ok
}
