package gimmecdn

import (
	"archive/zip"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/api"
	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/model"
	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/spi"
	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

type cdnEngine struct {
	objectStorage spi.ObjectStorage
}

var re = regexp.MustCompile(`^[a-zA-Z0-9-_]+`)

func NewCDNEngine(objectStorage spi.ObjectStorage) api.CDNEngine {
	return &cdnEngine{
		objectStorage: objectStorage,
	}
}

// filterArray filter objects array
func (cdn *cdnEngine) filterArray(arr []model.ObjectInfos, fileName string, version string) []model.ObjectInfos {
	var filtered []model.ObjectInfos
	for _, item := range arr {
		if strings.Contains(item.Key, fileName) && strings.Contains(item.Key, version) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterArray get package version
func (cdn *cdnEngine) getVersion(objStorageFile string) string {
	return strings.Split(strings.Split(objStorageFile, "@")[1], "/")[0]
}

// getLatestVersion get last package version
func (cdn *cdnEngine) getLatestVersion(arr []model.ObjectInfos) string {
	var versions []string
	for _, curr := range arr {
		versions = append(versions, cdn.getVersion(curr.Key))
	}
	semver.Sort(versions)
	return versions[len(versions)-1]
}

// getLatestPackagePath get latest package path
func (cdn *cdnEngine) getLatestPackagePath(pkg string, version string, fileName string) string {
	objs := cdn.objectStorage.ListObjects(fmt.Sprintf("%s@%s", pkg, version))
	filtred := cdn.filterArray(objs, fileName, version)

	if len(filtred) == 0 {
		return fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	}

	lversion := cdn.getLatestVersion(filtred)
	return fmt.Sprintf("%s@%s%s", pkg, lversion, fileName)
}

func (cdn *cdnEngine) CreatePackage(name string, version string, file io.ReaderAt, fileSize int64) *errors.GimmeError {
	archive, err := zip.NewReader(file, fileSize)
	if err != nil {
		logrus.Error("[UploadManager] ArchiveProcessor - Error while reading zip file", err)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while reading zip file"))
	}

	folderName := fmt.Sprintf("%s@%s", name, version)

	if exists := cdn.objectStorage.ObjectExists(folderName); exists {
		return errors.NewBusinessError(errors.Conflict, fmt.Errorf("the package %v already exists", folderName))
	}

	nbFiles := len(archive.File)

	var wg sync.WaitGroup
	wg.Add(nbFiles)

	for _, currentFile := range archive.File {
		go func(currentFile *zip.File) {
			defer wg.Done()
			logrus.Debug("[UploadManager] ArchiveProcessor - Unzipping file ", currentFile.Name)
			fileName := re.ReplaceAllString(currentFile.FileHeader.Name, folderName)
			err := cdn.objectStorage.AddObject(fileName, currentFile)
			if err != nil {
				logrus.Errorf("[UploadManager] ArchiveProcessor - Error while processing file %s", fileName)
			}
		}(currentFile)
	}

	wg.Wait()
	return nil
}

func (cdn *cdnEngine) GetFileFromPackage(pkg string, version string, fileName string) (*model.CDNObject, *errors.GimmeError) {
	valid := semver.IsValid(fmt.Sprintf("v%v", version))
	if !valid {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("invalid version (asked version must be semver compatible)"))
	}

	var objectPath string
	slice := strings.Split(version, ".")
	if len(slice) == 3 {
		objectPath = fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	} else {
		objectPath = cdn.getLatestPackagePath(pkg, version, fileName)
	}

	return cdn.objectStorage.GetObject(objectPath)
}

func (cdn *cdnEngine) GetPackageFiles(pkg string, version string) []model.ObjectInfos {
	objs := cdn.objectStorage.ListObjects(fmt.Sprintf("%s@%s", pkg, version))

	var files []model.ObjectInfos
	for _, object := range objs {
		files = append(files, model.ObjectInfos{
			Key:          object.Key,
			ContentType:  object.ContentType,
			Size:         object.Size,
			LastModified: object.LastModified,
		})
	}
	return files
}

func (cdn *cdnEngine) RemovePackage(pkg string, version string) *errors.GimmeError {
	err := cdn.objectStorage.RemoveObjects(fmt.Sprintf("%s@%s", pkg, version))
	if err != nil {
		return err
	}
	return nil
}
