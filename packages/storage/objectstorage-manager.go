package storage

import (
	"archive/zip"
	"context"
	"fmt"
	"io"

	"github.com/gimme-cdn/gimme/errors"

	fileutils "github.com/gimme-cdn/gimme/utils"
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
	CreateBucket(bucketName string, location string) *errors.GimmeError
	AddObject(objectName string, file *zip.File) *errors.GimmeError
	GetObject(objectName string) (*minio.Object, *errors.GimmeError)
	ObjectExists(objectName string) bool
}

// NewObjectStorageManager create a new object storage manager
func NewObjectStorageManager(client ObjectStorageClient) ObjectStorageManager {
	return &objectStorageManager{
		client: client,
		ctx:    context.Background(),
	}
}

// CreateBucket create a new bucket
func (osm *objectStorageManager) CreateBucket(bucketName string, location string) *errors.GimmeError {
	err := osm.client.MakeBucket(osm.ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := osm.client.BucketExists(osm.ctx, bucketName)
		if errBucketExists == nil && exists {
			logrus.Infof("We already own %s\n", bucketName)
		} else {
			logrus.Error("[ObjectStorageManager] CreateBucket - Fail to create bucket", err)
			return errors.NewError(errors.InternalError, fmt.Errorf("fail to create bucket %v in location %v", bucketName, location))
		}
	}

	logrus.Infof("Bucket successfully created %s\n", bucketName)
	osm.bucketName = bucketName
	osm.location = location
	return nil
}

// AddObject add object into the bucket
func (osm *objectStorageManager) AddObject(objectName string, file *zip.File) *errors.GimmeError {
	// Skip dir
	if file.FileInfo().IsDir() {
		return nil
	}

	src, err := file.Open()
	if err != nil {
		logrus.Error("[ObjectStorageManager] AddObject - Fail to open input file", err)
		return errors.NewError(errors.InternalError, fmt.Errorf("addObject - Fail to open input file"))
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
		return errors.NewError(errors.InternalError, fmt.Errorf("fail to put object %s in the object storage", objectName))
	}

	logrus.Debugf("Successfully uploaded %s of size %d\n", objectName, info.Size)
	return nil
}

// GetObject get object from the bucket
func (osm *objectStorageManager) GetObject(objectName string) (*minio.Object, *errors.GimmeError) {
	object, err := osm.client.GetObject(osm.ctx, osm.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logrus.Error("[ObjectStorageManager] GetObject - Fail to get object from the object storage", err)
		return nil, errors.NewError(errors.InternalError, fmt.Errorf("fail to get object %s from the object storage", objectName))
	}
	return object, nil
}

// ObjectExists return if object exists in bucket or not
func (osm *objectStorageManager) ObjectExists(objectName string) bool {
	objectCh := osm.client.ListObjects(osm.ctx, osm.bucketName, minio.ListObjectsOptions{
		Prefix:    objectName,
		Recursive: true,
	})

	for object := range objectCh {
		if len(object.ETag) > 0 {
			return true
		}
	}
	return false
}
