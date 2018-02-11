package goroutinechecker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rwool/ex/test/helpers/goroutinechecker"
)

func TestGoroutineChecker(t *testing.T) {
	defer goroutinechecker.New(t, false)()
	t.Run("SubTest", func(t2 *testing.T) {
		defer goroutinechecker.New(t2, true)
	})
}

func TestGetStack(t *testing.T) {
	assert.True(t, len(goroutinechecker.GetStack()) > 0, "got empty stack trace")
}
