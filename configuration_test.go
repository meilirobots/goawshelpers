package goawshelpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_convertKeynameToPath(t *testing.T) {
	key := "HELLO_WORLD"
	path := convertKeynameToPath(key, "dev", "_")
	assert.Equal(t, "/dev/hello/world", path)
}

func Test_convertPathToKeyname(t *testing.T) {
	path := "/dev/hello/world"
	assert.Equal(t, convertPathToKeyname(path, "dev", "."), "hello.world")
}
