package content

import (
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gimme-cdn/gimme/internal/upload"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

type ContentService struct {
	objectStorageManager storage.ObjectStorageManager
}

type File struct {
	Name   string
	Size   int64
	Folder bool
}

// NewContentService create a new content service instance
func NewContentService(objectStorageManager storage.ObjectStorageManager) ContentService {
	return ContentService{
		objectStorageManager,
	}
}

// filterArray filter objects array
func (svc *ContentService) filterArray(arr []minio.ObjectInfo, fileName string, version string) []minio.ObjectInfo {
	var filtered []minio.ObjectInfo
	for _, item := range arr {
		if strings.Contains(item.Key, fileName) && strings.Contains(item.Key, version) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterArray get package version
func (svc *ContentService) getVersion(objStorageFile string) string {
	return strings.Split(strings.Split(objStorageFile, "@")[1], "/")[0]
}

// getLatestVersion get last package version
func (svc *ContentService) getLatestVersion(arr []minio.ObjectInfo) string {
	var versions []string
	for _, curr := range arr {
		versions = append(versions, svc.getVersion(curr.Key))
	}
	semver.Sort(versions)
	return versions[len(versions)-1]
}

// getLatestPackagePath get latest package path
func (svc *ContentService) getLatestPackagePath(pkg string, version string, fileName string) string {
	objs := svc.objectStorageManager.ListObjects(fmt.Sprintf("%s@%s", pkg, version))
	filtred := svc.filterArray(objs, fileName, version)

	if len(filtred) == 0 {
		return fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	}

	lversion := svc.getLatestVersion(filtred)
	return fmt.Sprintf("%s@%s%s", pkg, lversion, fileName)
}

// UploadPackage upload package
func (svc *ContentService) UploadPackage(name string, version string, archive *multipart.FileHeader) *errors.GimmeError {
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

	uploadErr := upload.ArchiveProcessor(name, version, svc.objectStorageManager, reader, archive.Size)
	return uploadErr
}

// GetFile get package file
func (svc *ContentService) GetFile(pkg string, version string, fileName string) (*minio.Object, *errors.GimmeError) {
	valid := semver.IsValid(fmt.Sprintf("v%v", version))
	if !valid {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("invalid version"))
	}

	var objectPath string
	slice := strings.Split(version, ".")
	if len(slice) == 3 {
		objectPath = fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	} else {
		objectPath = svc.getLatestPackagePath(pkg, version, fileName)
	}

	return svc.objectStorageManager.GetObject(objectPath)
}

// GetFiles get package files
func (svc *ContentService) GetFiles(pkg string, version string) ([]File, *errors.GimmeError) {
	objs := svc.objectStorageManager.ListObjects(fmt.Sprintf("%s@%s", pkg, version))

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

// DeletePackage delete package
func (svc *ContentService) DeletePackage(pkg string, version string) *errors.GimmeError {
	err := svc.objectStorageManager.RemoveObjects(fmt.Sprintf("%s@%s", pkg, version))
	if err != nil {
		return err
	}
	return nil
}
