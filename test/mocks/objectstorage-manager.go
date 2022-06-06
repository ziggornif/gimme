package mocks

import (
	"archive/zip"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManager struct{}

func (osc *MockOSManager) CreateBucket(_ string, _ string) *errors.GimmeError {
	return nil
}
func (osc *MockOSManager) AddObject(_ string, _ *zip.File) *errors.GimmeError {
	return nil
}
func (osc *MockOSManager) GetObject(_ string) (*minio.Object, *errors.GimmeError) {
	return &minio.Object{}, nil
}

func (osc *MockOSManager) ObjectExists(_ string) bool {
	return false
}

func (osc *MockOSManager) ListObjects(_ string) []minio.ObjectInfo {
	return []minio.ObjectInfo{}
}

func (osc *MockOSManager) RemoveObjects(_ string) *errors.GimmeError {
	return nil
}
