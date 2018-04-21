package sshtarget_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"testing"

	errors2 "github.com/pkg/errors"
	"github.com/rwool/ex/ex/internal/sshtarget"
	"github.com/rwool/ex/test/helpers/comperr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type testCloser struct {
	Err error
}

func (tc *testCloser) Close() error {
	return tc.Err
}

func TestGetKnownHostsPaths(t *testing.T) {
	t.Parallel()

	u, err := user.Current()
	require.NoError(t, err, "unable to get current user")
	require.NotEmpty(t, u.HomeDir, "got zero length user name")
	userKH := fmt.Sprintf("%s/.ssh/known_hosts", u.HomeDir)

	testOpener := func(t *testing.T, e1, e2 error, c1, c2 *testCloser, numCalls int) ([]string, error) {
		var call int
		opener := func(s string) (io.Closer, error) {
			call++
			if call == 1 {
				assert.Equal(t, userKH, s,
					"unexpected user known hosts path")
				if e1 != nil {
					return nil, e1
				}
				return c1, nil
			}
			assert.Equal(t, sshtarget.GlobalKnownHosts, s,
				"unexpected global known_hosts path")
			if e2 != nil {
				return nil, e2
			}
			return c2, nil
		}
		paths, err := sshtarget.GetKnownHostPaths(opener)
		assert.Equal(t, numCalls, call, "too many calls to file opener")
		return paths, err
	}

	tcs := []struct {
		Name             string
		E1               error
		E2               error
		CloseErr1        error
		CloseErr2        error
		NumCalls         int
		ExpectedPaths    []string
		ExpectedErrCause error
	}{
		{
			Name:          "Both Files Accessible",
			NumCalls:      2,
			ExpectedPaths: []string{sshtarget.GlobalKnownHosts, userKH},
		},
		{
			Name:          "User KH Only",
			E2:            errors.New("file does not exist"),
			NumCalls:      2,
			ExpectedPaths: []string{userKH},
		},
		{
			Name:          "Global KH Only",
			E1:            errors.New("file does not exist"),
			NumCalls:      2,
			ExpectedPaths: []string{sshtarget.GlobalKnownHosts},
		},
		{
			Name:             "Neither KH Only",
			E1:               errors.New("file does not exist"),
			E2:               errors.New("file does not exist"),
			NumCalls:         2,
			ExpectedPaths:    nil,
			ExpectedErrCause: errors.New("file does not exist"),
		},
		{
			Name:             "Error Closing User",
			CloseErr1:        errors.New("i/o error"),
			NumCalls:         1,
			ExpectedPaths:    nil,
			ExpectedErrCause: errors.New("i/o error"),
		},
		{
			Name:             "Error Closing Global",
			CloseErr2:        errors.New("i/o error"),
			NumCalls:         2,
			ExpectedPaths:    nil,
			ExpectedErrCause: errors.New("i/o error"),
		},
	}

	for _, tCase := range tcs {
		tc := tCase
		t.Run(tc.Name, func(t2 *testing.T) {
			t2.Parallel()
			tc1, tc2 := &testCloser{Err: tc.CloseErr1}, &testCloser{Err: tc.CloseErr2}
			paths, err := testOpener(t2, tc.E1, tc.E2, tc1, tc2, tc.NumCalls)
			comperr.RequireEqualErr(t2, tc.ExpectedErrCause, errors2.Cause(err),
				"unexpected error from test case")
			assert.Equal(t2, tc.ExpectedPaths, paths, "unexpected paths returned")
		})
	}
}

type testWriteSeeker struct {
	bytes.Buffer
	writeErr error
	seekErr  error
}

func (tws *testWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	if tws.seekErr != nil {
		return 0, tws.seekErr
	}
	return 0, nil
}

func (tws *testWriteSeeker) Write(p []byte) (int, error) {
	if tws.writeErr != nil {
		return 0, tws.writeErr
	}
	return tws.Buffer.Write(p)
}

func generateKey(keyType interface{}) ssh.PublicKey {
	pkey := func(key interface{}) ssh.PublicKey {
		k, err := ssh.NewPublicKey(key)
		if err != nil {
			panic(err)
		}
		return k
	}

	switch keyType.(type) {
	case rsa.PublicKey:
		priv, err := rsa.GenerateKey(rand.Reader, 256)
		if err != nil {
			panic(err)
		}
		return pkey(priv.Public())
	case ecdsa.PublicKey:
		priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			panic(err)
		}
		return pkey(priv.Public())
	default:
		panic("unsupported")
	}
}

func TestAddToKnownHosts(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		Name               string
		Marker             sshtarget.KnownHostsMarker
		WriteErr           error
		SeekErr            error
		Hosts              []string
		PublicKey          ssh.PublicKey
		DisableHashing     bool
		ExpectedErrorCause error
	}{
		{
			Name:      "Basic Host RSA",
			Hosts:     []string{"127.0.0.1"},
			PublicKey: generateKey(rsa.PublicKey{}),
		},
		{
			Name:      "Two Hosts RSA",
			Hosts:     []string{"127.0.0.1", "192.168.0.1"},
			PublicKey: generateKey(rsa.PublicKey{}),
		},
		{
			Name:      "Two Hosts With Ports RSA",
			Hosts:     []string{"127.0.0.1:22", "192.168.0.1:99"},
			PublicKey: generateKey(rsa.PublicKey{}),
		},
		{
			Name:      "Two Hosts With Ports ECDSA",
			Hosts:     []string{"127.0.0.1:22", "192.168.0.1:99"},
			PublicKey: generateKey(ecdsa.PublicKey{}),
		},
		{
			Name:      "Two Hosts With Ports ECDSA Revoked",
			Marker:    sshtarget.MarkerRevoked,
			Hosts:     []string{"127.0.0.1:22", "192.168.0.1:99"},
			PublicKey: generateKey(ecdsa.PublicKey{}),
		},
		{
			Name:           "Two Hosts With Ports RSA No Hashing",
			Hosts:          []string{"127.0.0.1:22", "192.168.0.1:99"},
			DisableHashing: true,
			PublicKey:      generateKey(rsa.PublicKey{}),
		},
		{
			Name:      "IPv6 with Port RSA",
			Hosts:     []string{"[::1]:99"},
			PublicKey: generateKey(rsa.PublicKey{}),
		},
		{
			Name:      "IPv6 with Port RSA Cert Authority",
			Marker:    sshtarget.MarkerCertAuthority,
			Hosts:     []string{"[::1]:99"},
			PublicKey: generateKey(rsa.PublicKey{}),
		},
		{
			Name:      "IPv6 with Port ECDSA",
			Hosts:     []string{"[::1]:99"},
			PublicKey: generateKey(ecdsa.PublicKey{}),
		},
		{
			Name:           "IPv6 with Port RSA No Hashing",
			Hosts:          []string{"[::1]:99"},
			DisableHashing: true,
			PublicKey:      generateKey(rsa.PublicKey{}),
		},
		{
			Name:               "Bad Seek",
			Hosts:              []string{"127.0.0.1"},
			SeekErr:            errors.New("i/o error"),
			PublicKey:          generateKey(rsa.PublicKey{}),
			ExpectedErrorCause: errors.New("i/o error"),
		},
		{
			Name:               "Bad Write",
			Hosts:              []string{"127.0.0.1"},
			WriteErr:           errors.New("i/o error"),
			PublicKey:          generateKey(rsa.PublicKey{}),
			ExpectedErrorCause: errors.New("i/o error"),
		},
	}

	for _, tCase := range tcs {
		tc := tCase
		t.Run(tc.Name, func(t2 *testing.T) {
			t2.Parallel()
			tws := &testWriteSeeker{
				writeErr: tc.WriteErr,
				seekErr:  tc.SeekErr,
			}
			err := sshtarget.AddToKnownHosts(tws, tc.Hosts, tc.PublicKey, tc.DisableHashing, tc.Marker)
			comperr.AssertEqualErr(t2, tc.ExpectedErrorCause, errors2.Cause(err),
				"unexpected error adding host line")
			if err != nil {
				return
			}

			marker, hosts, pubKey, comment, _, err := ssh.ParseKnownHosts(tws.Bytes())
			if tc.Marker == sshtarget.MarkerNone {
				assert.Empty(t2, marker, "unexpected marker where none expected")
			} else {
				// ParseKnownHosts removes the leading @ symbol.
				assert.Equal(t2, tc.Marker.String()[1:], marker, "unexpected marker")
			}
			require.NoError(t2, err, "bad known_hosts line")
			assert.Len(t2, hosts, len(tc.Hosts), "unexpected number of hosts")
			assert.Equal(t2, tc.PublicKey.Type(), pubKey.Type(), "unexpected public key type")
			assert.Empty(t2, comment, "unexpected comment")
			if !tc.DisableHashing {
				assert.Equal(t2, len(tc.Hosts), bytes.Count(tws.Bytes(), []byte("|1|")),
					"unexpected number of HASH_MAGICs")
			}
		})
	}
}

func TestKnownHostsFile(t *testing.T) {
	t.Parallel()
	f, err := ioutil.TempFile("", "my_temp")
	require.NoError(t, err, "Temp file creation does not fail.")
	defer os.Remove(f.Name())
	defer f.Close()

	cb, err := sshtarget.KnownHostsFilesCallback(f.Name())
	require.NoError(t, err, "Known host file callback creation does not fail.")

	rsaPrivKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err, "Private key generation does not fail.")

	sshPubKey, err := ssh.NewPublicKey(rsaPrivKey.Public())
	require.NoError(t, err, "SSH public key creation does not fail.")

	hostname := "localhost:22"
	hostIP := "127.0.0.1"
	port := 22
	err = cb(hostname, &net.TCPAddr{
		IP:   net.ParseIP(hostIP),
		Port: port,
	}, sshPubKey)

	// Unknown host.
	require.True(t, sshtarget.IsUnknownHost(err), "Host must be unknown")

	// Known host.
	err = sshtarget.AddToKnownHosts(f, []string{hostIP}, sshPubKey, true, sshtarget.MarkerNone)
	require.NoError(t, err, "known_hosts host addition does not fail.")
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err, "Seek to file beginning does not fail.")
	cb, err = sshtarget.KnownHostsFilesCallback(f.Name())
	require.NoError(t, err, "Known host file callback creation does not fail.")
	err = cb(hostname, &net.TCPAddr{
		IP:   net.ParseIP(hostIP),
		Port: port,
	}, sshPubKey)
	require.NoError(t, err, "Host key verification with known host succeeds.")

	// Bad key.
	err = f.Truncate(0)
	require.NoError(t, err, "known_hosts trucation must succeed.")
	rsaPrivKey2, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err, "Private key generation does not fail.")
	sshPubKey2, err := ssh.NewPublicKey(rsaPrivKey2.Public())
	err = sshtarget.AddToKnownHosts(f, []string{hostIP}, sshPubKey2, true, sshtarget.MarkerNone)
	require.NoError(t, err, "known_hosts host addition does not fail.")
	cb, err = sshtarget.KnownHostsFilesCallback(f.Name())
	require.NoError(t, err, "Known host file callback creation does not fail.")
	err = cb(hostname, &net.TCPAddr{
		IP:   net.ParseIP(hostIP),
		Port: port,
	}, sshPubKey)
	require.True(t, sshtarget.IsKeyChange(err), "Key for host change detected.")

	// Revoked key.
	err = f.Truncate(0)
	require.NoError(t, err, "known_hosts trucation must succeed.")
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err, "Seek to file beginning does not fail.")
	err = sshtarget.AddToKnownHosts(f, []string{hostIP}, sshPubKey, true, sshtarget.MarkerRevoked)
	require.NoError(t, err, "known_hosts host addition does not fail.")
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err, "Seek to file beginning does not fail.")
	cb, err = sshtarget.KnownHostsFilesCallback(f.Name())
	require.NoError(t, err, "Known host file callback creation does not fail.")
	err = cb(hostname, &net.TCPAddr{
		IP:   net.ParseIP(hostIP),
		Port: port,
	}, sshPubKey)
	require.True(t, sshtarget.IsRevoked(err), "Key revocation detected.")
}
