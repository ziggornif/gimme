package persistence

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisClient_InvalidURL(t *testing.T) {
	_, err := NewRedisClient("://not-a-url")
	assert.Error(t, err)
}

func TestNewRedisClient_Unreachable(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	addr := mr.Addr()
	mr.Close()

	_, err = NewRedisClient("redis://" + addr)
	assert.Error(t, err)
}

func TestNewRedisClient_OK(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, err := NewRedisClient("redis://" + mr.Addr())
	require.NoError(t, err)
	t.Cleanup(client.CloseConnection)

	assert.NotNil(t, client.GetClient())
}

func TestRedisClient_CloseConnection_Nil(t *testing.T) {
	rc := &RedisClient{client: nil}
	assert.NotPanics(t, func() {
		rc.CloseConnection()
	})
}
