package mocks

import (
	"archive/zip"
	"fmt"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerErr struct{}

func (osc *MockOSManagerErr) CreateBucket(bucketName string, location string) error {
	return fmt.Errorf("boom")
}
func (osc *MockOSManagerErr) AddObject(objectName string, file *zip.File) error {
	return fmt.Errorf("boom")
}
func (osc *MockOSManagerErr) GetObject(objectName string) (*minio.Object, error) {
	return nil, fmt.Errorf("boom")
}
