package content

import (
	"archive/zip"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"math"

	"github.com/sirupsen/logrus"
)

func fileIntegrity(file *zip.File) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer func(src io.ReadCloser) {
		if closeErr := src.Close(); closeErr != nil {
			logrus.Errorf("[ContentService] fileIntegrity - Fail to close %s", file.Name)
		}
	}(src)

	declared := file.UncompressedSize64
	if declared >= math.MaxInt64 {
		return "", fmt.Errorf("declared uncompressed size for %s is out of range: %d", file.Name, declared)
	}

	h := sha512.New384()
	size, err := io.Copy(h, io.LimitReader(src, int64(declared)+1))
	if err != nil {
		return "", err
	}
	if size < 0 || uint64(size) != declared {
		return "", fmt.Errorf("unexpected uncompressed size for %s: got %d, want %d", file.Name, size, declared)
	}

	return "sha384-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
