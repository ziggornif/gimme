package archive_validator

import (
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFile(t *testing.T) {
	header := make(map[string][]string)
	header["Content-Type"] = append(header["Content-Type"], "application/zip")
	err := ValidateFile(&multipart.FileHeader{
		Header: header,
	})
	assert.Nil(t, err)
}

func TestValidateFileEmpty(t *testing.T) {
	err := ValidateFile(nil)
	assert.Equal(t, "input file is required. (accepted types : application/zip)", err.String())
}

func TestValidateFileErr(t *testing.T) {
	header := make(map[string][]string)
	header["Content-Type"] = append(header["Content-Type"], "invalid")
	err := ValidateFile(&multipart.FileHeader{
		Header: header,
	})
	assert.Equal(t, "invalid input file type. (accepted types : application/zip)", err.String())
}
