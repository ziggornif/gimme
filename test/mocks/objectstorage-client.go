package mocks

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type MockOSClient struct{}

func (osc *MockOSClient) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	return nil
}

func (osc *MockOSClient) BucketExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (osc *MockOSClient) PutObject(_ context.Context, _ string, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{Size: 10}, nil
}
func (osc *MockOSClient) GetObject(_ context.Context, _ string, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	return &minio.Object{}, nil
}

func (osc *MockOSClient) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, 1)
	defer close(ch)
	ch <- minio.ObjectInfo{ETag: "test"}
	return ch
}

func (osc *MockOSClient) RemoveObjects(_ context.Context, _ string, _ <-chan minio.ObjectInfo, _ minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	ch := make(chan minio.RemoveObjectError, 1)
	defer close(ch)
	return ch
}
