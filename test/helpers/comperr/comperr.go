// Package comperr implements assertions for checking error values.
package comperr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertEqualErr compares the strings of the errors.
//
// This is a helper function to work around the assert packages inability to deal
// with comparison to nil errors easily.
func AssertEqualErr(t *testing.T, expected, actual error, msgAndArgs ...interface{}) {
	if expected == nil {
		assert.NoError(t, actual, msgAndArgs...)
	} else {
		assert.EqualError(t, actual, expected.Error(), msgAndArgs...)
	}
}

// RequireEqualErr compares the strings of the errors.
//
// This is a helper function to work around the assert packages inability to deal
// with comparison to nil errors easily.
func RequireEqualErr(t *testing.T, expected, actual error, msgAndArgs ...interface{}) {
	if expected == nil {
		require.NoError(t, actual, msgAndArgs...)
	} else {
		require.EqualError(t, actual, expected.Error(), msgAndArgs...)
	}
}
