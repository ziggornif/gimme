package mocks

import (
	"archive/zip"
	"context"
	"fmt"

	"github.com/ziggornif/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerErr struct{}

func (osc *MockOSManagerErr) CreateBucket(_ context.Context, _ string, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) AddObject(_ context.Context, _ string, _ *zip.File, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
func (osc *MockOSManagerErr) AddBytes(_ context.Context, _ string, _ []byte, _ string, _ string) *errors.GimmeError {
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

func (osc *MockOSManagerErr) ListObjectsPage(_ context.Context, _ string, _ string, _ int) ([]minio.ObjectInfo, bool) {
	return nil, false
}

func (osc *MockOSManagerErr) ListCommonPrefixes(_ context.Context, _ string) []string {
	return nil
}

func (osc *MockOSManagerErr) RemoveObjects(_ context.Context, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}

func (osc *MockOSManagerErr) Ping(_ context.Context) *errors.GimmeError {
	return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
}
