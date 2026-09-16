package mocks

import (
	"archive/zip"
	"context"
	"fmt"

	"github.com/ziggornif/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManagerExists struct{}

func (osc *MockOSManagerExists) CreateBucket(_ context.Context, _ string, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("boom"))
}
func (osc *MockOSManagerExists) AddObject(_ context.Context, _ string, _ *zip.File, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("boom"))
}
func (osc *MockOSManagerExists) AddBytes(_ context.Context, _ string, _ []byte, _ string, _ string) *errors.GimmeError {
	return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("boom"))
}
func (osc *MockOSManagerExists) GetObject(_ context.Context, _ string) (*minio.Object, *errors.GimmeError) {
	return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("boom"))
}

func (osc *MockOSManagerExists) ObjectExists(_ context.Context, _ string) bool {
	return true
}

func (osc *MockOSManagerExists) ListObjects(_ context.Context, _ string) []minio.ObjectInfo {
	return []minio.ObjectInfo{}
}

func (osc *MockOSManagerExists) ListObjectsPage(_ context.Context, _ string, _ string, _ int) ([]minio.ObjectInfo, bool) {
	return nil, false
}

func (osc *MockOSManagerExists) ListCommonPrefixes(_ context.Context, _ string) []string {
	return nil
}

func (osc *MockOSManagerExists) RemoveObjects(_ context.Context, _ string) *errors.GimmeError {
	return nil
}

func (osc *MockOSManagerExists) Ping(_ context.Context) *errors.GimmeError {
	return nil
}
