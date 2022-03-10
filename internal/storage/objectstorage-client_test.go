package storage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gimme-cdn/gimme/configs"
)

func TestNewObjectStorageClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client, err := NewObjectStorageClient(&configs.Configuration{
		S3Url:      strings.Split(srv.URL, "http://")[1],
		S3SSL:      false,
		S3Key:      "test",
		S3Secret:   "test",
		S3Location: "eu-west-1",
	})

	assert.NotEmpty(t, client)
	assert.Nil(t, err)
}

func TestNewObjectStorageClientErr(t *testing.T) {
	_, err := NewObjectStorageClient(&configs.Configuration{})
	assert.Equal(t, "error while create object storage client", err.String())
}
