package storage

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/metrics"

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
	Delete RemoveKindErrorEnum = "Delete"
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
	Ping(ctx context.Context) *errors.GimmeError
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

	start := time.Now()
	defer func() {
		metrics.S3OperationDuration.WithLabelValues("AddObject").Observe(time.Since(start).Seconds())
	}()

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

// GetObject get object from the bucket.
// The histogram measures time-to-first-byte (TTFB): the Minio client returns a lazy
// *minio.Object and actual data streaming happens when the caller reads from it.
func (osm *objectStorageManager) GetObject(ctx context.Context, objectName string) (*minio.Object, *errors.GimmeError) {
	start := time.Now()
	defer func() {
		metrics.S3OperationDuration.WithLabelValues("GetObject").Observe(time.Since(start).Seconds())
	}()

	object, err := osm.client.GetObject(ctx, osm.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logrus.Error("[ObjectStorageManager] GetObject - Fail to get object from the object storage", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to get object %s from the object storage", objectName))
	}
	return object, nil
}

// ListObjects list objects in parent
func (osm *objectStorageManager) ListObjects(ctx context.Context, objectParentName string) []minio.ObjectInfo {
	start := time.Now()
	defer func() {
		metrics.S3OperationDuration.WithLabelValues("ListObjects").Observe(time.Since(start).Seconds())
	}()

	var objects []minio.ObjectInfo
	objectCh := osm.client.ListObjects(ctx, osm.bucketName, minio.ListObjectsOptions{
		Prefix:    objectParentName,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			logrus.Errorf("[ObjectStorageManager] ListObjects - Error reading object from storage: %s", object.Err.Error())
			continue
		}
		objects = append(objects, object)
	}
	return objects
}

// ObjectExists return if object exists in bucket or not.
// It uses an exact prefix match: the listed object key must start with
// objectName+"/" (a directory) or equal objectName exactly, so that "1.0.0"
// does not match "1.0.0-beta".
func (osm *objectStorageManager) ObjectExists(ctx context.Context, objectName string) bool {
	start := time.Now()
	defer func() {
		metrics.S3OperationDuration.WithLabelValues("ObjectExists").Observe(time.Since(start).Seconds())
	}()

	// Append "/" so the prefix only matches the exact package@version directory
	// and not versions that share a common prefix (e.g. "1.0.0" vs "1.0.0-beta").
	exactPrefix := objectName + "/"

	objectCh := osm.client.ListObjects(ctx, osm.bucketName, minio.ListObjectsOptions{
		Prefix:    exactPrefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			logrus.Errorf("[ObjectStorageManager] ObjectExists - Error reading object from storage: %s", object.Err.Error())
			continue
		}
		// Any object without an error confirms the prefix exists.
		return true
	}
	return false
}

// Ping checks that the object storage is reachable by verifying the bucket exists
func (osm *objectStorageManager) Ping(ctx context.Context) *errors.GimmeError {
	start := time.Now()
	defer func() {
		metrics.S3OperationDuration.WithLabelValues("Ping").Observe(time.Since(start).Seconds())
	}()

	exists, err := osm.client.BucketExists(ctx, osm.bucketName)
	if err != nil {
		logrus.Error("[ObjectStorageManager] Ping - Fail to reach object storage", err)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("object storage is unreachable"))
	}
	if !exists {
		logrus.Errorf("[ObjectStorageManager] Ping - Bucket %s does not exist", osm.bucketName)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("bucket %s does not exist", osm.bucketName))
	}
	return nil
}

// RemoveObjects remove objects from storage
func (osm *objectStorageManager) RemoveObjects(ctx context.Context, objectParentName string) *errors.GimmeError {
	start := time.Now()
	defer func() {
		metrics.S3OperationDuration.WithLabelValues("RemoveObjects").Observe(time.Since(start).Seconds())
	}()

	objectsCh := make(chan minio.ObjectInfo)

	var (
		removeErrors []RemoveError
		mu           sync.Mutex
	)

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
				mu.Lock()
				removeErrors = append(removeErrors, RemoveError{
					Kind:       Read,
					ObjectName: object.Key,
					Details:    object.Err.Error(),
				})
				mu.Unlock()
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
		mu.Lock()
		removeErrors = append(removeErrors, RemoveError{
			Kind:       Delete,
			ObjectName: rErr.ObjectName,
			Details:    details,
		})
		mu.Unlock()
	}

	if len(removeErrors) != 0 {
		return errors.NewBusinessError(
			errors.InternalError,
			fmt.Errorf("error while removing objects. Details : %v", removeErrors),
		)
	}

	return nil
}
