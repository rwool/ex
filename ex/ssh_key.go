package ex

import "github.com/rwool/ex/ex/internal/sshtarget"

// SSHHostKeyCallback is a function for validating a host's SSH key.
type SSHHostKeyCallback = sshtarget.HostKeyCallback

// SSHFixedHostKey is a function for verifying that a host's key matches a
// single, fixed key.
var SSHFixedHostKey = sshtarget.FixedHostKey

// SSHInsecureIgnoreHostKey is a function for skipping verification of a
// host's key.
var SSHInsecureIgnoreHostKey = sshtarget.InsecureIgnoreHostKey
