package mocks

import (
	"archive/zip"
	"fmt"

	"github.com/gimme-cdn/gimme/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerExists struct{}

func (osc *MockOSManagerExists) CreateBucket(bucketName string, location string) *errors.GimmeError {
	return errors.NewError(errors.BadRequest, fmt.Errorf("boom"))
}
func (osc *MockOSManagerExists) AddObject(objectName string, file *zip.File) *errors.GimmeError {
	return errors.NewError(errors.BadRequest, fmt.Errorf("boom"))
}
func (osc *MockOSManagerExists) GetObject(objectName string) (*minio.Object, *errors.GimmeError) {
	return nil, errors.NewError(errors.BadRequest, fmt.Errorf("boom"))
}

func (osc *MockOSManagerExists) ObjectExists(objectName string) bool {
	return true
}
