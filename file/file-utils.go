package file

import (
	"archive/zip"
	"mime"
	"path/filepath"
)

func GetFileContentType(file *zip.File) (string, error) {
	contentType := mime.TypeByExtension(filepath.Ext(file.FileHeader.Name))
	return contentType, nil
}
