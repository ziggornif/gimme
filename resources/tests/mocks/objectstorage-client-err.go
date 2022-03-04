package mocks

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

type MockOSClientErr struct{}

func (osc *MockOSClientErr) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return fmt.Errorf("boom")
}

func (osc *MockOSClientErr) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return false, fmt.Errorf("boom")
}

func (osc *MockOSClientErr) PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{}, fmt.Errorf("boom")
}
func (osc *MockOSClientErr) GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return nil, fmt.Errorf("boom")
}

func (osc *MockOSClientErr) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, 1)
	return ch
}
