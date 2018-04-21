package ex

import (
	"io"

	"github.com/rwool/ex/ex/internal/sshtarget"
	"golang.org/x/crypto/ssh"
)

// KnownHostsMarker represents the usage of a marker in a known_hosts file.
type KnownHostsMarker uint8

// Markers available to use with lines in the known_hosts file(s).
const (
	MarkerNone KnownHostsMarker = iota
	MarkerCertAuthority
	MarkerRevoked
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
	return sshtarget.AddToKnownHosts(f, hosts, pubKey, disableHashing, sshtarget.KnownHostsMarker(marker))
}

// DefaultKnownHosts returns the standard known_hosts file paths
// for the known_hosts files that are found.
func DefaultKnownHosts() ([]string, error) {
	return sshtarget.DefaultKnownHosts()
}

// KnownHostsFilesCallback creates a host key callback using the hosts
// specified in the given known_hosts files.
//
// To support adding host keys dynamically, the returned HostKeyCallback may
// be wrapped in a function to support handling the returned errors from the
// callback.
func KnownHostsFilesCallback(knownHostsPaths ...string) (ssh.HostKeyCallback, error) {
	return sshtarget.KnownHostsFilesCallback(knownHostsPaths...)
}

// IsUnknownHost indicates if the given error was due to an unknown host.
func IsUnknownHost(e error) bool {
	return sshtarget.IsUnknownHost(e)
}

// IsKeyChange indicates if the given error was due to an unexpected host
// key for an already known host.
//
// If this returns true, then it may be an indication of a
// man-in-the-middle attack.
func IsKeyChange(e error) bool {
	return sshtarget.IsKeyChange(e)
}

// IsRevoked indicates if the given error was due to a revoked host key
// being given.
func IsRevoked(e error) bool {
	return sshtarget.IsRevoked(e)
}
