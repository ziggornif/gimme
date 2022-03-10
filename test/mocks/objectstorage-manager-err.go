package mocks

import (
	"archive/zip"
	"fmt"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerErr struct{}

func (osc *MockOSManagerErr) CreateBucket(bucketName string, location string) *errors.GimmeError {
	return errors.NewError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) AddObject(objectName string, file *zip.File) *errors.GimmeError {
	return errors.NewError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) GetObject(objectName string) (*minio.Object, *errors.GimmeError) {
	return nil, errors.NewError(errors.InternalError, fmt.Errorf("boom"))
}

func (osc *MockOSManagerErr) ObjectExists(objectName string) bool {
	return false
}

func (osc *MockOSManagerErr) ListObjects(objectName string) []minio.ObjectInfo {
	return []minio.ObjectInfo{}
}
