package content

import (
	"crypto/sha512"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileIntegrity(t *testing.T) {
	data := []byte("console.log('integrity');\n")
	digest := sha512.Sum384(data)
	expected := "sha384-" + base64.StdEncoding.EncodeToString(digest[:])

	actual, err := fileIntegrity(zipEntry(t, "app.js", data))
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
