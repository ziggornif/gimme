package mocks

import (
	"archive/zip"

	"github.com/minio/minio-go/v7"
)

type MockOSManager struct{}

func (osc *MockOSManager) CreateBucket(bucketName string, location string) error {
	return nil
}
func (osc *MockOSManager) AddObject(objectName string, file *zip.File) error {
	return nil
}
func (osc *MockOSManager) GetObject(objectName string) (*minio.Object, error) {
	return &minio.Object{}, nil
}
