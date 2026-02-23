package mocks

import (
	"archive/zip"
	"context"
	"fmt"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerErr struct{}

func (osc *MockOSManagerErr) CreateBucket(_ context.Context, _ string, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) AddObject(_ context.Context, _ string, _ *zip.File) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) GetObject(_ context.Context, _ string) (*minio.Object, *errors.GimmeError) {
	return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}

func (osc *MockOSManagerErr) ObjectExists(_ context.Context, _ string) bool {
	return false
}

func (osc *MockOSManagerErr) ListObjects(_ context.Context, _ string) []minio.ObjectInfo {
	return []minio.ObjectInfo{}
}

func (osc *MockOSManagerErr) RemoveObjects(_ context.Context, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
