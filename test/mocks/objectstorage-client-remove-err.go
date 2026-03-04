package mocks

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

// MockOSClientRemoveErr is an ObjectStorageClient that returns a RemoveObjectError
// with a non-nil Err field, exercising the rErr.Err != nil branch in RemoveObjects.
type MockOSClientRemoveErr struct{}

func (osc *MockOSClientRemoveErr) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	return nil
}

func (osc *MockOSClientRemoveErr) BucketExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (osc *MockOSClientRemoveErr) PutObject(_ context.Context, _ string, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{}, nil
}

func (osc *MockOSClientRemoveErr) GetObject(_ context.Context, _ string, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	return &minio.Object{}, nil
}

func (osc *MockOSClientRemoveErr) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, 1)
	defer close(ch)
	ch <- minio.ObjectInfo{Key: "test-object"}
	return ch
}

// RemoveObjects returns a RemoveObjectError with a non-nil Err to exercise the
// rErr.Err != nil branch in objectStorageManager.RemoveObjects.
func (osc *MockOSClientRemoveErr) RemoveObjects(_ context.Context, _ string, _ <-chan minio.ObjectInfo, _ minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	ch := make(chan minio.RemoveObjectError, 1)
	defer close(ch)
	ch <- minio.RemoveObjectError{
		ObjectName: "test-object",
		Err:        fmt.Errorf("delete failed"),
	}
	return ch
}
