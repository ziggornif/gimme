package content

import (
	"context"
	"os"
	"testing"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/test/mocks"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentService_CreatePackage(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	assert.Nil(t, err)
}

func TestContentService_CreatePackageZipErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})

	fileName := "../../resources/tests/test.zip"
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, 1)
	assert.Equal(t, "error while reading zip file", err.Error())
}

func TestContentService_CreatePackageUploadErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerErr{})

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	require.NotNil(t, err)
	assert.Equal(t, errors.ErrorKindEnum(errors.InternalError), err.Kind)
}

func TestContentService_CreatePackageExists(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerExists{})

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	assert.Equal(t, "the package test@1.0.0 already exists", err.Error())
}

func TestContentService_GetFileSemverErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})
	_, err := service.GetFile(context.Background(), "test", "a.b.c", "test.js")
	assert.Equal(t, "invalid version (asked version must be semver compatible)", err.Error())
}

func TestContentService_GetFile(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})
	file, err := service.GetFile(context.Background(), "test", "1.1.1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetMajorFile(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})
	file, err := service.GetFile(context.Background(), "test", "1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetMinorFile(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})
	file, err := service.GetFile(context.Background(), "test", "1.1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetFiles(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})
	files, err := service.GetFiles(context.Background(), "test", "1.1.1")
	assert.Equal(t, 2, len(files))
	assert.Nil(t, err)
}

func TestContentService_DeletePackage(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Nil(t, err)
}

func TestContentService_DeletePackageErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerErr{})
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Equal(t, "boom", err.Error())
}

func TestContentService_GetLatestVersionEmpty(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{})
	result := service.getLatestVersion([]minio.ObjectInfo{})
	assert.Equal(t, "", result)
}
