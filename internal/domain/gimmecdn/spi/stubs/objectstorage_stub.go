package stubs

import (
	"archive/zip"
	"fmt"
	"strings"
	"time"

	fileutils "github.com/gimme-cdn/gimme/pkg/file-utils"

	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/spi"

	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/model"
	"github.com/gimme-cdn/gimme/internal/errors"
)

type objectsMemory struct {
	Name        string
	ObjectInfos model.ObjectInfos
	Object      model.CDNObject
}
type stub struct {
	objects []objectsMemory
}

func (s *stub) CreateBucket(bucketName string, location string) *errors.GimmeError {
	//TODO implement me
	panic("implement me")
}

func (s *stub) ListObjects(objectParentName string) []model.ObjectInfos {
	var objectsInfos []model.ObjectInfos
	for _, val := range s.objects {
		objectsInfos = append(objectsInfos, val.ObjectInfos)
	}
	return objectsInfos
}

func (s *stub) GetObject(objectName string) (*model.CDNObject, *errors.GimmeError) {
	for _, val := range s.objects {
		if val.Name == objectName {
			return &val.Object, nil
		}
	}
	return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("Not found"))
}

func (s *stub) ObjectExists(objectName string) bool {
	for _, val := range s.objects {
		if strings.Contains(val.Name, objectName) {
			return true
		}
	}
	return false
}

func (s *stub) AddObject(objectName string, file *zip.File) *errors.GimmeError {
	contentType := fileutils.GetFileContentType(file)
	reader, _ := file.Open()
	s.objects = append(s.objects, objectsMemory{
		Name: objectName,
		ObjectInfos: model.ObjectInfos{
			Key:          objectName,
			Size:         file.FileInfo().Size(),
			ContentType:  contentType,
			LastModified: time.Now(),
		},
		Object: model.CDNObject{
			File:        reader,
			ContentType: contentType,
			Size:        file.FileInfo().Size(),
		},
	})
	return nil
}

func (s *stub) RemoveObjects(objectParentName string) *errors.GimmeError {
	//TODO implement me
	panic("implement me")
}

func NewObjectStorageStub() spi.ObjectStorage {
	return &stub{}
}
