package publish

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "css"), 0o755))
	files := map[string]string{"app.js": "app", "css/a.css": "css", ".gimmeignore": "*.map"}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	path, err := Archive(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(path) })
	reader, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	actual := map[string]string{}
	for _, entry := range reader.File {
		file, openErr := entry.Open()
		require.NoError(t, openErr)
		data, readErr := io.ReadAll(file)
		require.NoError(t, readErr)
		require.NoError(t, file.Close())
		actual[entry.Name] = string(data)
	}
	assert.Equal(t, files, actual)
}

func TestArchiveErrors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := Archive(t.TempDir())
		assert.ErrorContains(t, err, "no files to publish")
	})
	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(path, nil, 0o644))
		_, err := Archive(path)
		assert.ErrorContains(t, err, "not a directory")
	})
	t.Run("missing", func(t *testing.T) { _, err := Archive(filepath.Join(t.TempDir(), "missing")); assert.Error(t, err) })
}

func TestArchiveSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}
	t.Run("outside", func(t *testing.T) {
		dir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside")
		require.NoError(t, os.WriteFile(outside, []byte("x"), 0o644))
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))
		_, err := Archive(dir)
		assert.Error(t, err)
	})
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "target"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "target", "x"), []byte("x"), 0o644))
		require.NoError(t, os.Symlink("target", filepath.Join(dir, "link")))
		_, err := Archive(dir)
		assert.ErrorContains(t, err, "symlink to directory")
	})
}
