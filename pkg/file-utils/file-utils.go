package file_utils

import (
	"archive/zip"
	"mime"
	"path/filepath"
)

func GetFileContentType(file *zip.File) string {
	contentType := mime.TypeByExtension(filepath.Ext(file.FileHeader.Name))
	return contentType
}
