package storage

import (
	"fmt"

	credentials "github.com/aws/aws-sdk-go-v2/aws/credentials"
	"github.com/aws/aws-sdk-go-v2/aws/session"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/gimme-cdn/gimme/configs"
	//"github.com/minio/minio-go/v7/pkg/credentials"
	//
	//"github.com/minio/minio-go/v7"
)

type ObjectStorageClient interface {
	CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error)
	WaitUntilBucketExists(input *s3.HeadBucketInput) error
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	ListObjects(input *s3.ListObjectsInput) (*s3.ListObjectsOutput, error)
	DeleteObjects(input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error)
	//MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	//BucketExists(ctx context.Context, bucketName string) (bool, error)
	//PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	//GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	//ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	//RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError
}

// NewObjectStorageClient create a new object storage client
func NewObjectStorageClient(config *configs.Configuration) (*s3.S3, *errors.GimmeError) {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.S3Key, config.S3Secret, ""),
		Endpoint:         aws.String(config.S3Url),
		Region:           config.S3Location,
		DisableSSL:       aws.Bool(!config.S3SSL),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		logrus.Error("[ObjectStorageClient] NewObjectStorageClient - Error while create object storage client", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while create object storage client"))
	}

	s3Client := s3.New(newSession)

	return s3Client, nil
}
