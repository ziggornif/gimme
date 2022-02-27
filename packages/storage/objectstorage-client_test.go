package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gimme-cli/gimme/config"
)

func TestNewObjectStorageClient(t *testing.T) {
	_, err := NewObjectStorageClient(&config.Configuration{})
	assert.Equal(t, "Error while create object storage client", err.Error())
}
