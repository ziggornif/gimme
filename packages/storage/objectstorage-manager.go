package storage

import (
	"archive/zip"
	"context"
	"fmt"
	"io"

	fileutils "github.com/gimme-cli/gimme/utils"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

type objectStorageManager struct {
	ctx        context.Context
	client     ObjectStorageClient
	bucketName string
	location   string
}

type ObjectStorageManager interface {
	CreateBucket(bucketName string, location string) error
	AddObject(objectName string, file *zip.File) error
	GetObject(objectName string) (*minio.Object, error)
}

// NewObjectStorageManager create a new object storage manager
func NewObjectStorageManager(client ObjectStorageClient) ObjectStorageManager {
	return &objectStorageManager{
		client: client,
		ctx:    context.Background(),
	}
}

// CreateBucket create a new bucket
func (osm *objectStorageManager) CreateBucket(bucketName string, location string) error {
	err := osm.client.MakeBucket(osm.ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := osm.client.BucketExists(osm.ctx, bucketName)
		if errBucketExists == nil && exists {
			logrus.Infof("We already own %s\n", bucketName)
		} else {
			logrus.Error("[ObjectStorageManager] CreateBucket - Fail to create bucket", err)
			return fmt.Errorf("Fail to create bucket %v in location %v", bucketName, location)
		}
	}

	logrus.Infof("Bucket successfully created %s\n", bucketName)
	osm.bucketName = bucketName
	osm.location = location
	return nil
}

// CreateBucket add object into the bucket
func (osm *objectStorageManager) AddObject(objectName string, file *zip.File) error {
	// Skip dir
	if file.FileInfo().IsDir() {
		return nil
	}

	src, err := file.Open()
	if err != nil {
		logrus.Error("[ObjectStorageManager] AddObject - Fail to open input file", err)
		return fmt.Errorf("AddObject - Fail to open input file")
	}

	defer func(src io.ReadCloser) {
		err := src.Close()
		if err != nil {
			logrus.Error("AddObject - Fail to close file")
		}
	}(src)

	contentType := fileutils.GetFileContentType(file)
	if len(contentType) == 0 {
		contentType = "text/plain"
	}

	info, err := osm.client.PutObject(osm.ctx, osm.bucketName, objectName, src, file.FileInfo().Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		logrus.Error("[ObjectStorageManager] AddObject - Fail to put object in the object storage", err)
		return fmt.Errorf("Fail to put object %s in the object storage", objectName)
	}

	logrus.Debugf("Successfully uploaded %s of size %d\n", objectName, info.Size)
	return nil
}

// GetObject get object from the bucket
func (osm *objectStorageManager) GetObject(objectName string) (*minio.Object, error) {
	object, err := osm.client.GetObject(osm.ctx, osm.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logrus.Error("[ObjectStorageManager] GetObject - Fail to get object from the object storage", err)
		return nil, fmt.Errorf("Fail to get object %s from the object storage", objectName)
	}
	return object, nil
}
