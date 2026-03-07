package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPGClient_InvalidURL(t *testing.T) {
	_, err := NewPGClient("://not-a-url")
	assert.Error(t, err)
}

func TestNewPGClient_Unreachable(t *testing.T) {
	_, err := NewPGClient("postgres://localhost:0/nonexistent")
	assert.Error(t, err)
}

func TestPGClient_CloseConnection_Nil(t *testing.T) {
	pgc := &PGClient{pool: nil}
	assert.NotPanics(t, func() {
		pgc.CloseConnection()
	})
}
