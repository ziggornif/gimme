package mocks

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

type MockOSClientBucketExists struct{}

func (osc *MockOSClientBucketExists) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	return fmt.Errorf("boom")
}

func (osc *MockOSClientBucketExists) BucketExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (osc *MockOSClientBucketExists) PutObject(_ context.Context, _ string, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{Size: 10}, nil
}
func (osc *MockOSClientBucketExists) GetObject(_ context.Context, _ string, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	return &minio.Object{}, nil
}

func (osc *MockOSClientBucketExists) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, 1)
	defer close(ch)
	return ch
}

func (osc *MockOSClientBucketExists) RemoveObjects(_ context.Context, _ string, _ <-chan minio.ObjectInfo, _ minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	ch := make(chan minio.RemoveObjectError, 1)
	defer close(ch)
	return ch
}
