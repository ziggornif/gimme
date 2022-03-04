package mocks

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

type MockOSClientBucketExists struct{}

func (osc *MockOSClientBucketExists) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return fmt.Errorf("boom")
}

func (osc *MockOSClientBucketExists) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return true, nil
}

func (osc *MockOSClientBucketExists) PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{Size: 10}, nil
}
func (osc *MockOSClientBucketExists) GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return &minio.Object{}, nil
}

func (osc *MockOSClientBucketExists) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, 1)
	return ch
}
