package mocks

import (
	"archive/zip"
	"fmt"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerErr struct{}

func (osc *MockOSManagerErr) CreateBucket(_ string, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) AddObject(_ string, _ *zip.File) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) GetObject(_ string) (*minio.Object, *errors.GimmeError) {
	return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}

func (osc *MockOSManagerErr) ObjectExists(_ string) bool {
	return false
}

func (osc *MockOSManagerErr) ListObjects(_ string) []minio.ObjectInfo {
	return []minio.ObjectInfo{}
}

func (osc *MockOSManagerErr) RemoveObjects(_ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
