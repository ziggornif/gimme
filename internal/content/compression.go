package content

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/sirupsen/logrus"
)

type Encoding string

const (
	EncodingBrotli Encoding = "br"
	EncodingGzip   Encoding = "gzip"

	compressionMinSize  = 1024
	compressionMaxSize  = 8 << 20
	compressionMinRatio = 0.8
	brotliQuality       = 5
	gzipLevel           = gzip.BestCompression
)

func (encoding Encoding) suffix() string {
	if encoding == EncodingBrotli {
		return ".br"
	}
	if encoding == EncodingGzip {
		return ".gz"
	}
	return ""
}

func compressibleContentType(name string) (string, bool) {
	extension := strings.ToLower(filepath.Ext(name))
	contentType := mime.TypeByExtension(extension)
	if contentType == "" && extension == ".map" {
		contentType = "application/json"
	}
	if contentType == "" {
		return "", false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType, false
	}
	if strings.HasPrefix(mediaType, "text/") {
		return contentType, true
	}
	switch mediaType {
	case "application/javascript", "application/json", "application/manifest+json", "application/xml", "application/xhtml+xml", "image/svg+xml", "application/wasm":
		return contentType, true
	default:
		return contentType, false
	}
}

func compressVariants(file *zip.File) (map[Encoding][]byte, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func(src io.ReadCloser) {
		if closeErr := src.Close(); closeErr != nil {
			logrus.Errorf("[ContentService] compressVariants - Fail to close %s", file.Name)
		}
	}(src)

	var brotliBuffer, gzipBuffer bytes.Buffer
	brotliWriter := brotli.NewWriterLevel(&brotliBuffer, brotliQuality)
	gzipWriter, err := gzip.NewWriterLevel(&gzipBuffer, gzipLevel)
	if err != nil {
		return nil, err
	}
	size, copyErr := io.Copy(io.MultiWriter(brotliWriter, gzipWriter), io.LimitReader(src, compressionMaxSize+1))
	brotliErr := brotliWriter.Close()
	gzipErr := gzipWriter.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if brotliErr != nil {
		return nil, brotliErr
	}
	if gzipErr != nil {
		return nil, gzipErr
	}
	if size > compressionMaxSize {
		return nil, nil
	}

	variants := make(map[Encoding][]byte, 2)
	if float64(brotliBuffer.Len()) <= compressionMinRatio*float64(size) {
		variants[EncodingBrotli] = brotliBuffer.Bytes()
	}
	if float64(gzipBuffer.Len()) <= compressionMinRatio*float64(size) {
		variants[EncodingGzip] = gzipBuffer.Bytes()
	}
	return variants, nil
}
