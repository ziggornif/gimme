package storage

import (
	"archive/zip"
	"context"
	"io"

	fileutils "github.com/gimme-cli/gimme/utils"

	"github.com/gimme-cli/gimme/config"

	"github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

type ObjectStorageManager struct {
	ctx         context.Context
	minioClient *minio.Client
	bucketName  string
	location    string
}

func NewObjectStorageManager(config *config.Configuration) (*ObjectStorageManager, error) {
	minioClient, err := minio.New(config.S3Url, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3Key, config.S3Secret, ""),
		Secure: config.S3SSL,
	})

	if err != nil {
		return nil, err
	}

	return &ObjectStorageManager{
		minioClient: minioClient,
		ctx:         context.Background(),
	}, nil
}

func (osm *ObjectStorageManager) CreateBucket(bucketName string, location string) error {
	err := osm.minioClient.MakeBucket(osm.ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := osm.minioClient.BucketExists(osm.ctx, bucketName)
		if errBucketExists == nil && exists {
			logrus.Infof("We already own %s\n", bucketName)
		} else {
			return err
		}
	}

	logrus.Infof("Bucket successfully created %s\n", bucketName)
	osm.bucketName = bucketName
	osm.location = location
	return nil
}

func (osm *ObjectStorageManager) AddObject(objectName string, file *zip.File) error {
	// Skip dir
	if file.FileInfo().IsDir() {
		return nil
	}

	src, _ := file.Open()
	defer func(src io.ReadCloser) {
		err := src.Close()
		if err != nil {
			logrus.Error("[ObjectStorageManager] AddObject - Fail to close file")
		}
	}(src)

	contentType := fileutils.GetFileContentType(file)
	if len(contentType) == 0 {
		contentType = "text/plain"
	}

	info, err := osm.minioClient.PutObject(osm.ctx, osm.bucketName, objectName, src, file.FileInfo().Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return err
	}

	logrus.Debugf("Successfully uploaded %s of size %d\n", objectName, info.Size)
	return nil
}

func (osm *ObjectStorageManager) GetObject(objectName string) (*minio.Object, error) {
	object, err := osm.minioClient.GetObject(osm.ctx, osm.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return object, nil
}
