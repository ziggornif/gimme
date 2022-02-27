package mocks

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type MockOSClient struct{}

func (osc *MockOSClient) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return nil
}

func (osc *MockOSClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return false, nil
}

func (osc *MockOSClient) PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{Size: 10}, nil
}
func (osc *MockOSClient) GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return &minio.Object{}, nil
}
