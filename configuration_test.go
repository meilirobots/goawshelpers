package goawshelpers

import (
	"fmt"
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

func Test_SSMConfiguration(t *testing.T) {
	config, err := NewSSMConfiguration(SSMConfigurationInit{
		UseEnvParams: false,
	})

	assert.Nil(t, config)
	assert.NotNil(t, err)
	assert.Contains(t, fmt.Sprint(err), "no awsAccessKey and/or awsSecretAccessKey provided")
}

func Test_BiConfiguration(t *testing.T) {
	c := EnvironmentConfiguration{
		UseUpper: true,
	}

	b, err := NewBiConfiguration(c, nil)

	assert.Nil(t, err)
	assert.NotNil(t, b)

	val, err := b.Get("nonexistingkeyshouldgohere")

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprint(err), "no value with key nonexistingkeyshouldgohere")
	assert.Equal(t, val, "")

	err = b.Delete("random key that does nothing")
	assert.Nil(t, err)
}
