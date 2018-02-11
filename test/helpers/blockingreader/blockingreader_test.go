package blockingreader

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rwool/ex/test/helpers/goroutinechecker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockingReader_Cancel(t *testing.T) {
	defer goroutinechecker.New(t, false)()

	h := strings.NewReader("hello")
	br := NewBlockingReader(3*time.Second, h)

	start := time.Now()
	buf := make([]byte, 20)
	read, err := br.Read(buf)
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.True(t, elapsed > 3*time.Second && elapsed < 4*time.Second,
		"read blocked for wrong amount of time")
	assert.Equal(t, "hello", string(buf[:read]), "unexpected output")

	read, err = br.Read(buf)
	assert.EqualError(t, err, io.EOF.Error())
	assert.Equal(t, 0, read)
}
