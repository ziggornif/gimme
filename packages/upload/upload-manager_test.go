package upload

import (
	"mime/multipart"
	"os"
	"testing"

	"github.com/gimme-cli/gimme/resources/tests/mocks"

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

func TestValidateFileErr(t *testing.T) {
	header := make(map[string][]string)
	header["Content-Type"] = append(header["Content-Type"], "invalid")
	err := ValidateFile(&multipart.FileHeader{
		Header: header,
	})
	assert.Equal(t, "Invalid input file type. (accepted types : application/zip)", err.String())
}

func TestArchiveProcessor(t *testing.T) {
	fileName := "../../resources/tests/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := ArchiveProcessor("test", "1.0.0", &mocks.MockOSManager{}, reader, size)
	assert.Nil(t, err)
}

func TestArchiveProcessorZipErr(t *testing.T) {
	fileName := "../../resources/tests/test.zip"
	reader, _ := os.Open(fileName)
	err := ArchiveProcessor("test", "1.0.0", &mocks.MockOSManager{}, reader, 1)
	assert.Equal(t, "Error while reading zip file", err.String())
}

func TestArchiveProcessorUploadErr(t *testing.T) {
	fileName := "../../resources/tests/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := ArchiveProcessor("test", "1.0.0", &mocks.MockOSManagerErr{}, reader, size)
	assert.Nil(t, err) // error is silent here
}
