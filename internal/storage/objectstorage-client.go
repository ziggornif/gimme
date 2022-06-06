package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/sirupsen/logrus"

	"github.com/gimme-cdn/gimme/configs"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/minio/minio-go/v7"
)

type ObjectStorageClient interface {
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError
}

// NewObjectStorageClient create a new object storage client
func NewObjectStorageClient(config *configs.Configuration) (ObjectStorageClient, *errors.GimmeError) {
	minioClient, err := minio.New(config.S3Url, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3Key, config.S3Secret, ""),
		Secure: config.S3SSL,
	})

	if err != nil {
		logrus.Error("[ObjectStorageClient] NewObjectStorageClient - Error while create object storage client", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while create object storage client"))
	}

	return minioClient, nil
}
