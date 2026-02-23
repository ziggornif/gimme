package storage

import (
	"archive/zip"
	"context"
	"fmt"
	"io"

	"github.com/gimme-cdn/gimme/internal/errors"

	fileutils "github.com/gimme-cdn/gimme/pkg/file-utils"

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
	client     ObjectStorageClient
	bucketName string
	location   string
}

type RemoveKindErrorEnum string

const (
	Read   RemoveKindErrorEnum = "Read"
	Delete                     = "Delete"
)

type RemoveError struct {
	Kind       RemoveKindErrorEnum
	ObjectName string
	Details    string
}

type ObjectStorageManager interface {
	CreateBucket(ctx context.Context, bucketName string, location string) *errors.GimmeError
	AddObject(ctx context.Context, objectName string, file *zip.File) *errors.GimmeError
	GetObject(ctx context.Context, objectName string) (*minio.Object, *errors.GimmeError)
	ObjectExists(ctx context.Context, objectName string) bool
	ListObjects(ctx context.Context, objectParentName string) []minio.ObjectInfo
	RemoveObjects(ctx context.Context, objectParentName string) *errors.GimmeError
}

// NewObjectStorageManager create a new object storage manager
func NewObjectStorageManager(client ObjectStorageClient) ObjectStorageManager {
	return &objectStorageManager{
		client: client,
	}
}

// CreateBucket create a new bucket
func (osm *objectStorageManager) CreateBucket(ctx context.Context, bucketName string, location string) *errors.GimmeError {
	err := osm.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := osm.client.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			logrus.Infof("We already own %s\n", bucketName)
		} else {
			logrus.Error("[ObjectStorageManager] CreateBucket - Fail to create bucket", err)
			return errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to create bucket %v in location %v", bucketName, location))
		}
	}

	logrus.Infof("Bucket successfully created %s\n", bucketName)
	osm.bucketName = bucketName
	osm.location = location
	return nil
}

// AddObject add object into the bucket
func (osm *objectStorageManager) AddObject(ctx context.Context, objectName string, file *zip.File) *errors.GimmeError {
	// Skip dir
	if file.FileInfo().IsDir() {
		return nil
	}

	src, err := file.Open()
	if err != nil {
		logrus.Error("[ObjectStorageManager] AddObject - Fail to open input file", err)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("addObject - Fail to open input file"))
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

	info, err := osm.client.PutObject(ctx, osm.bucketName, objectName, src, file.FileInfo().Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		logrus.Error("[ObjectStorageManager] AddObject - Fail to put object in the object storage", err)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to put object %s in the object storage", objectName))
	}

	logrus.Debugf("Successfully uploaded %s of size %d\n", objectName, info.Size)
	return nil
}

// GetObject get object from the bucket
func (osm *objectStorageManager) GetObject(ctx context.Context, objectName string) (*minio.Object, *errors.GimmeError) {
	object, err := osm.client.GetObject(ctx, osm.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logrus.Error("[ObjectStorageManager] GetObject - Fail to get object from the object storage", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to get object %s from the object storage", objectName))
	}
	return object, nil
}

// ListObjects list objects in parent
func (osm *objectStorageManager) ListObjects(ctx context.Context, objectParentName string) []minio.ObjectInfo {
	var objects []minio.ObjectInfo
	objectCh := osm.client.ListObjects(ctx, osm.bucketName, minio.ListObjectsOptions{
		Prefix:    objectParentName,
		Recursive: true,
	})

	for object := range objectCh {
		objects = append(objects, object)
	}
	return objects
}

// ObjectExists return if object exists in bucket or not
func (osm *objectStorageManager) ObjectExists(ctx context.Context, objectName string) bool {
	objectCh := osm.client.ListObjects(ctx, osm.bucketName, minio.ListObjectsOptions{
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

// RemoveObjects remove objects from storage
func (osm *objectStorageManager) RemoveObjects(ctx context.Context, objectParentName string) *errors.GimmeError {
	objectsCh := make(chan minio.ObjectInfo)

	var removeErrors []RemoveError

	// Send object names that are needed to be removed to objectsCh
	go func() {
		defer close(objectsCh)
		// List all objects from a bucket-name with a matching prefix.
		for object := range osm.client.ListObjects(ctx, osm.bucketName, minio.ListObjectsOptions{
			Prefix:    objectParentName,
			Recursive: true,
		}) {
			if object.Err != nil {
				logrus.Errorf("[ObjectStorageManager] RemoveObjects - Fail to read objects from the object storage: %s", object.Err.Error())
				removeErrors = append(removeErrors, RemoveError{
					Kind:       Read,
					ObjectName: object.Key,
					Details:    object.Err.Error(),
				})
			} else {
				objectsCh <- object
			}
		}
	}()

	opts := minio.RemoveObjectsOptions{
		GovernanceBypass: true,
	}

	for rErr := range osm.client.RemoveObjects(ctx, osm.bucketName, objectsCh, opts) {
		details := ""
		if rErr.Err != nil {
			details = rErr.Err.Error()
		}
		logrus.Errorf("[ObjectStorageManager] RemoveObjects - Error detected during deletion: %s %s", rErr.ObjectName, details)
		removeErrors = append(removeErrors, RemoveError{
			Kind:       Delete,
			ObjectName: rErr.ObjectName,
			Details:    details,
		})
	}

	if len(removeErrors) != 0 {
		return errors.NewBusinessError(
			errors.InternalError,
			fmt.Errorf("error while removing objects. Details : %v", removeErrors),
		)
	}

	return nil
}
