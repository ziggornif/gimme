package storage

import (
	"context"
	"io"

	"github.com/gimme-cli/gimme/config"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/minio/minio-go/v7"
)

type ObjectStorageClient interface {
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
}

// NewObjectStorageClient create a new object storage client
func NewObjectStorageClient(config *config.Configuration) (ObjectStorageClient, error) {
	minioClient, err := minio.New(config.S3Url, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3Key, config.S3Secret, ""),
		Secure: config.S3SSL,
	})

	if err != nil {
		return nil, err
	}

	return minioClient, nil
}
