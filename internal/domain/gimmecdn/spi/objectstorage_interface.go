package spi

import (
	"archive/zip"

	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/model"
	"github.com/gimme-cdn/gimme/internal/errors"
)

type ObjectStorage interface {
	CreateBucket(bucketName string, location string) *errors.GimmeError
	ListObjects(objectParentName string) []model.ObjectInfos
	GetObject(objectName string) (*model.CDNObject, *errors.GimmeError)
	ObjectExists(objectName string) bool
	AddObject(objectName string, file *zip.File) *errors.GimmeError
	RemoveObjects(objectParentName string) *errors.GimmeError
}
