package api

import (
	"io"

	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/model"
	"github.com/gimme-cdn/gimme/internal/errors"
)

type CDNEngine interface {
	CreatePackage(name string, version string, file io.ReaderAt, size int64) *errors.GimmeError
	GetFileFromPackage(pkg string, version string, fileName string) (*model.CDNObject, *errors.GimmeError)
	GetPackageFiles(pkg string, version string) []model.ObjectInfos
	RemovePackage(pkg string, version string) *errors.GimmeError
}
