package files

import (
	"archive/zip"
	"context"
	"log"
	"mime"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

type FilesManager struct {
	ctx         context.Context
	minioClient *minio.Client
	bucketName  string
	location    string
}

func NewFilesManager(config MinioConfig) (*FilesManager, error) {
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})

	if err != nil {
		return nil, err
	}

	return &FilesManager{
		minioClient: minioClient,
		ctx:         context.Background(),
	}, nil
}

func (fm *FilesManager) CreateBucket(bucketName string, location string) error {
	err := fm.minioClient.MakeBucket(fm.ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := fm.minioClient.BucketExists(fm.ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Printf("We already own %s\n", bucketName)
		} else {
			return err
		}
	}

	log.Printf("Successfully created %s\n", bucketName)
	fm.bucketName = bucketName
	fm.location = location
	return nil
}

func GetFileContentType(file *zip.File) (string, error) {
	contentType := mime.TypeByExtension(filepath.Ext(file.FileHeader.Name))
	return contentType, nil
}

func (fm *FilesManager) AddObject(objectName string, file *zip.File) error {
	// Skip dir
	if file.FileInfo().IsDir() {
		return nil
	}

	src, _ := file.Open()
	defer src.Close()

	contentType, _ := GetFileContentType(file)
	if len(contentType) == 0 {
		contentType = "text/plain"
	}

	info, err := fm.minioClient.PutObject(fm.ctx, fm.bucketName, objectName, src, file.FileInfo().Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return err
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)
	return nil
}

func (fm *FilesManager) GetObject(objectName string) (*minio.Object, error) {
	object, err := fm.minioClient.GetObject(fm.ctx, fm.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return object, nil
}
