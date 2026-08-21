package content

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"io"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func zipEntry(t *testing.T, name string, data []byte) *zip.File {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	return r.File[0]
}

func TestCompressibleContentType(t *testing.T) {
	for _, name := range []string{"app.js", "app.mjs", "app.css", "app.json", "app.svg", "app.html", "app.txt", "app.map", "app.wasm"} {
		_, ok := compressibleContentType(name)
		assert.True(t, ok, name)
	}
	for _, name := range []string{"app.png", "app.woff2", "app.zip", "app.gz", "app.br", "app.unknown"} {
		_, ok := compressibleContentType(name)
		assert.False(t, ok, name)
	}
}

func TestCompressVariants(t *testing.T) {
	source := bytes.Repeat([]byte("const answer = 42;\n"), 256)
	variants, err := compressVariants(zipEntry(t, "app.js", source))
	require.NoError(t, err)
	require.Len(t, variants, 2)

	for encoding, compressed := range variants {
		assert.Less(t, len(compressed), len(source))
		var reader io.Reader
		if encoding == EncodingBrotli {
			reader = brotli.NewReader(bytes.NewReader(compressed))
		} else {
			gzipReader, gzipErr := gzip.NewReader(bytes.NewReader(compressed))
			require.NoError(t, gzipErr)
			defer func() { _ = gzipReader.Close() }()
			reader = gzipReader
		}
		decoded, readErr := io.ReadAll(reader)
		require.NoError(t, readErr)
		assert.Equal(t, source, decoded)
	}
}

func TestCompressVariantsDropsIncompressibleData(t *testing.T) {
	source := make([]byte, 32<<10)
	_, err := rand.Read(source)
	require.NoError(t, err)
	variants, err := compressVariants(zipEntry(t, "random.txt", source))
	require.NoError(t, err)
	assert.Empty(t, variants)
}
