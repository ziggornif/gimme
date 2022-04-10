package mocks

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

type MockOSClientErr struct{}

func (osc *MockOSClientErr) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	return fmt.Errorf("boom")
}

func (osc *MockOSClientErr) BucketExists(_ context.Context, _ string) (bool, error) {
	return false, fmt.Errorf("boom")
}

func (osc *MockOSClientErr) PutObject(_ context.Context, _ string, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{}, fmt.Errorf("boom")
}
func (osc *MockOSClientErr) GetObject(_ context.Context, _ string, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	return nil, fmt.Errorf("boom")
}

func (osc *MockOSClientErr) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, 1)
	defer close(ch)
	return ch
}
