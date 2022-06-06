package mocks

import (
	"archive/zip"
	"fmt"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerExists struct{}

func (osc *MockOSManagerExists) CreateBucket(_ string, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("boom"))
}
func (osc *MockOSManagerExists) AddObject(_ string, _ *zip.File) *errors.GimmeError {
	return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("boom"))
}
func (osc *MockOSManagerExists) GetObject(_ string) (*minio.Object, *errors.GimmeError) {
	return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("boom"))
}

func (osc *MockOSManagerExists) ObjectExists(_ string) bool {
	return true
}

func (osc *MockOSManagerExists) ListObjects(_ string) []minio.ObjectInfo {
	return []minio.ObjectInfo{}
}

func (osc *MockOSManagerExists) RemoveObjects(_ string) *errors.GimmeError {
	return nil
}
