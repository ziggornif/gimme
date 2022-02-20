package utils

import (
	"archive/zip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFileContentType(t *testing.T) {
	file := zip.File{
		FileHeader: zip.FileHeader{
			Name: "awesomefile.js",
		},
	}
	contentType := GetFileContentType(&file)
	assert.Equal(t, "application/javascript", contentType)
}

func TestGetFileContentTypeNil(t *testing.T) {
	file := zip.File{
		FileHeader: zip.FileHeader{
			Name: "awesomefile.bad",
		},
	}
	contentType := GetFileContentType(&file)
	assert.Equal(t, "", contentType)
}
