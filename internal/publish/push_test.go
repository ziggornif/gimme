package publish

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushRequest(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("app"), 0o644))
	archive, err := Archive(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(archive) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/packages", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, "pkg", r.FormValue("name"))
		assert.Equal(t, "1.0.0", r.FormValue("version"))
		file, header, openErr := r.FormFile("file")
		require.NoError(t, openErr)
		defer func() { _ = file.Close() }()
		assert.Equal(t, "application/zip", header.Header.Get("Content-Type"))
		data, readErr := io.ReadAll(file)
		require.NoError(t, readErr)
		zipReader, zipErr := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
		require.NoError(t, zipErr)
		require.Len(t, zipReader.File, 1)
		assert.Equal(t, "app.js", zipReader.File[0].Name)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	require.NoError(t, Push(context.Background(), server.Client(), Options{URL: server.URL + "/", Token: "secret", Name: "pkg", Version: "1.0.0", ArchivePath: archive}))
}

func TestPushStatusMapping(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "x.zip")
	require.NoError(t, os.WriteFile(archive, []byte("x"), 0o644))
	tests := []struct {
		status     int
		body, want string
	}{
		{400, `{"error":"bad version"}`, "400 Bad Request: bad version"}, {401, "", "authentication failed (401)"},
		{409, `{"error":"already exists"}`, "versions are immutable"}, {413, `{"error":"limit"}`, "413 Request Entity Too Large: limit"},
		{413, "<html>large</html>", "client_max_body_size"}, {500, `{"error":"broken"}`, "500 Internal Server Error: broken"}, {502, strings.Repeat("x", 600), "…"},
	}
	for _, test := range tests {
		t.Run(fmt.Sprint(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			err := Push(context.Background(), server.Client(), Options{URL: server.URL, ArchivePath: archive})
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}
