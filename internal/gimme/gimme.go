package gimme

import (
	"fmt"
	"mime/multipart"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gimme-cdn/gimme/internal/upload"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

type GimmeService struct {
	objectStorageManager storage.ObjectStorageManager
}

type File struct {
	Name   string
	Size   int64
	Folder bool
}

func NewGimmeService(objectStorageManager storage.ObjectStorageManager) GimmeService {
	return GimmeService{
		objectStorageManager,
	}
}

func (gimme *GimmeService) filterArray(arr []minio.ObjectInfo, fileName string, version string) []minio.ObjectInfo {
	var filtered []minio.ObjectInfo
	for _, item := range arr {
		if strings.Contains(item.Key, fileName) && strings.Contains(item.Key, version) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (gimme *GimmeService) getVersion(objStorageFile string) string {
	return strings.Split(strings.Split(objStorageFile, "@")[1], "/")[0]
}

func (gimme *GimmeService) getLatestVersion(arr []minio.ObjectInfo) string {
	var versions []string
	for _, curr := range arr {
		versions = append(versions, gimme.getVersion(curr.Key))
	}
	semver.Sort(versions)
	return versions[len(versions)-1]
}

func (gimme *GimmeService) getLatestFilePath(pkg string, version string, fileName string) string {
	objs := gimme.objectStorageManager.ListObjects(fmt.Sprintf("%s@%s", pkg, version))
	filtred := gimme.filterArray(objs, fileName, version)

	if len(filtred) == 0 {
		return fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	}

	lversion := gimme.getLatestVersion(filtred)
	return fmt.Sprintf("%s@%s%s", pkg, lversion, fileName)
}

func (gimme *GimmeService) UploadPackage(name string, version string, archive *multipart.FileHeader) *errors.GimmeError {
	validationErr := upload.ValidateFile(archive)
	if validationErr != nil {
		return validationErr
	}

	reader, _ := archive.Open()
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {
			logrus.Error("Fail to close file")
		}
	}(reader)

	uploadErr := upload.ArchiveProcessor(name, version, gimme.objectStorageManager, reader, archive.Size)
	return uploadErr
}

func (gimme *GimmeService) GetFile(pkg string, version string, fileName string) (*minio.Object, *errors.GimmeError) {
	valid := semver.IsValid(fmt.Sprintf("v%v", version))
	if !valid {
		return nil, errors.NewError(errors.BadRequest, fmt.Errorf("invalid version"))
	}

	var objectPath string
	slice := strings.Split(version, ".")
	if len(slice) == 3 {
		objectPath = fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	} else {
		objectPath = gimme.getLatestFilePath(pkg, version, fileName)
	}

	return gimme.objectStorageManager.GetObject(objectPath)
}

func (gimme *GimmeService) GetFiles(pkg string, version string) ([]File, *errors.GimmeError) {
	objs := gimme.objectStorageManager.ListObjects(fmt.Sprintf("%s@%s", pkg, version))

	var files []File
	for _, obj := range objs {
		files = append(files, File{
			Name:   obj.Key,
			Size:   obj.Size,
			Folder: false,
		})
	}
	return files, nil
}
