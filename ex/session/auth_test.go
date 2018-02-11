package session

import (
	"testing"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/stretchr/testify/assert"
)

func TestAuthPassword(t *testing.T) {
	defer goroutinechecker.New(t, false)()

	assert.NotNil(t, PasswordAuth(""))
}
