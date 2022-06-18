package storage

import (
	"archive/zip"
	"context"
	"fmt"
	"io"

	file_utils "github.com/gimme-cdn/gimme/pkg/file-utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gimme-cdn/gimme/internal/errors"
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
	CreateBucket(bucketName string, location string) *errors.GimmeError
	AddObject(objectName string, file *zip.File) *errors.GimmeError
	GetObject(objectName string) (*s3.GetObjectOutput, *errors.GimmeError)
	ObjectExists(objectName string) (bool, *errors.GimmeError)
	ListObjects(objectParentName string) ([]*s3.Object, *errors.GimmeError)
	//RemoveObjects(objectParentName string) *errors.GimmeError
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
	_, err := osm.client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		aerr, _ := err.(awserr.Error)

		fmt.Println(aerr.Code())
		if code := aerr.Code(); code == s3.ErrCodeBucketAlreadyOwnedByYou {
			logrus.Infof("We already own %s\n", bucketName)
		} else {
			logrus.Error("[ObjectStorageManager] CreateBucket - Fail to create bucket", err)
			return errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to create bucket %v in location %v", bucketName, location))
		}
	}
	fmt.Println("wait for s3 bucket to exist")
	err = osm.client.WaitUntilBucketExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("bucket failed to materialize: %v", err))
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
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("addObject - Fail to open input file"))
	}

	defer func(src io.ReadCloser) {
		err := src.Close()
		if err != nil {
			logrus.Error("AddObject - Fail to close file")
		}
	}(src)

	contentType := file_utils.GetFileContentType(file)
	if len(contentType) == 0 {
		contentType = "text/plain"
	}

	//info, err := osm.client.PutObject(osm.ctx, osm.bucketName, objectName, src, file.FileInfo().Size(), minio.PutObjectOptions{ContentType: contentType})

	_, err = osm.client.PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(osm.bucketName),
		Key:           aws.String(objectName),
		Body:          aws.ReadSeekCloser(src),
		ContentLength: aws.Int64(file.FileInfo().Size()),
		ChecksumSHA256: file.FileInfo().
	})

	if err != nil {
		logrus.Error("[ObjectStorageManager] AddObject - Fail to put object in the object storage", err)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to put object %s in the object storage", objectName))
	}

	logrus.Debugf("Successfully uploaded %s\n", objectName)
	return nil
}

// GetObject get object from the bucket
func (osm *objectStorageManager) GetObject(objectName string) (*s3.GetObjectOutput, *errors.GimmeError) {
	object, err := osm.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(osm.bucketName),
		Key:    aws.String(objectName),
	})

	//object, err := osm.client.GetObject(osm.ctx, osm.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logrus.Error("[ObjectStorageManager] GetObject - Fail to get object from the object storage", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to get object %s from the object storage", objectName))
	}
	return object, nil
}

// ListObjects list objects in parent
func (osm *objectStorageManager) ListObjects(objectParentName string) ([]*s3.Object, *errors.GimmeError) {
	resp, err := osm.client.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(osm.bucketName),
		Prefix: aws.String(objectParentName),
	})

	if err != nil {
		logrus.Error("[ObjectStorageManager] GetObject - Fail to list objects from the object storage", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("fail to list child objects of %s from the object storage", objectParentName))
	}

	return resp.Contents, nil
}

// ObjectExists return if object exists in bucket or not
func (osm *objectStorageManager) ObjectExists(objectName string) (bool, *errors.GimmeError) {
	objects, err := osm.ListObjects(objectName)
	if err != nil {
		return false, err
	}

	for _, object := range objects {
		if len(*object.ETag) > 0 {
			return true, nil
		}
	}
	return false, nil
}

//// RemoveObjects remove objects from storage
//func (osm *objectStorageManager) RemoveObjects(objectParentName string) *errors.GimmeError {
//	objectsCh := make(chan minio.ObjectInfo)
//
//	var removeErrors []RemoveError
//
//	// Send object names that are needed to be removed to objectsCh
//	go func() {
//		defer close(objectsCh)
//		// List all objects from a bucket-name with a matching prefix.
//		for object := range osm.client.ListObjects(osm.ctx, osm.bucketName, minio.ListObjectsOptions{
//			Prefix:    objectParentName,
//			Recursive: true,
//		}) {
//			if object.Err != nil {
//				logrus.Error("[ObjectStorageManager] RemoveObjects - Fail to read objects from the object storage", object.Err.Error())
//				removeErrors = append(removeErrors, RemoveError{
//					Kind:       Read,
//					ObjectName: object.Key,
//					Details:    object.Err.Error(),
//				})
//			} else {
//				objectsCh <- object
//			}
//		}
//	}()
//
//	opts := minio.RemoveObjectsOptions{
//		GovernanceBypass: true,
//	}
//
//	for rErr := range osm.client.RemoveObjects(osm.ctx, osm.bucketName, objectsCh, opts) {
//		fmt.Println("[ObjectStorageManager] RemoveObjects - Error detected during deletion: ", rErr)
//		removeErrors = append(removeErrors, RemoveError{
//			Kind:       Delete,
//			ObjectName: rErr.ObjectName,
//			Details:    rErr.Err.Error(),
//		})
//	}
//
//	if len(removeErrors) != 0 {
//		return errors.NewBusinessError(
//			errors.InternalError,
//			fmt.Errorf("Error while removing objects. Details : %v", removeErrors),
//		)
//	}
//
//	return nil
//}
