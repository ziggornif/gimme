package publish

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Archive builds a temporary ZIP containing the files in dir.
func Archive(dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("cannot publish %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cannot publish %s: not a directory", dir)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	tmp, err := os.CreateTemp("", "gimme-cli-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temporary archive: %w", err)
	}
	path := tmp.Name()
	failed := true
	defer func() {
		_ = tmp.Close()
		if failed {
			_ = os.Remove(path)
		}
	}()

	zw := zip.NewWriter(tmp)
	count := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("read %s: %w", name, walkErr)
		}
		if name == "." || entry.IsDir() {
			return nil
		}
		file, openErr := root.Open(name)
		if openErr != nil {
			return fmt.Errorf("read %s: %w", name, openErr)
		}
		fileInfo, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			return fmt.Errorf("inspect %s: %w", name, statErr)
		}
		if fileInfo.IsDir() {
			_ = file.Close()
			return fmt.Errorf("symlink to directory is not supported: %s", name)
		}
		if !fileInfo.Mode().IsRegular() {
			_ = file.Close()
			return fmt.Errorf("unsupported file type: %s", name)
		}
		header, headerErr := zip.FileInfoHeader(fileInfo)
		if headerErr != nil {
			_ = file.Close()
			return fmt.Errorf("archive %s: %w", name, headerErr)
		}
		header.Name = filepath.ToSlash(name)
		header.Method = zip.Deflate
		writer, createErr := zw.CreateHeader(header)
		if createErr != nil {
			_ = file.Close()
			return fmt.Errorf("archive %s: %w", name, createErr)
		}
		if _, copyErr := io.Copy(writer, file); copyErr != nil {
			_ = file.Close()
			return fmt.Errorf("archive %s: %w", name, copyErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("close %s: %w", name, closeErr)
		}
		count++
		return nil
	})
	if err == nil && count == 0 {
		err = fmt.Errorf("no files to publish in %s", dir)
	}
	if closeErr := zw.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("close archive: %w", closeErr)
	}
	if closeErr := tmp.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("close archive file: %w", closeErr)
	}
	if err != nil {
		return "", err
	}
	failed = false
	return path, nil
}
