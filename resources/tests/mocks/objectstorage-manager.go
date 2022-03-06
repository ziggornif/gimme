package mocks

import (
	"archive/zip"

	"github.com/gimme-cdn/gimme/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManager struct{}

func (osc *MockOSManager) CreateBucket(bucketName string, location string) *errors.GimmeError {
	return nil
}
func (osc *MockOSManager) AddObject(objectName string, file *zip.File) *errors.GimmeError {
	return nil
}
func (osc *MockOSManager) GetObject(objectName string) (*minio.Object, *errors.GimmeError) {
	return &minio.Object{}, nil
}

func (osc *MockOSManager) ObjectExists(objectName string) bool {
	return false
}
