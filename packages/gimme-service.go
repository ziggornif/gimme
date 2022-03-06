package packages

import (
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"

	"github.com/gimme-cdn/gimme/errors"
	"github.com/gimme-cdn/gimme/packages/storage"

	"golang.org/x/mod/semver"
)

func filter(arr []minio.ObjectInfo, fileName string, version string) []minio.ObjectInfo {
	var filtred []minio.ObjectInfo
	for _, item := range arr {
		if strings.Contains(item.Key, fileName) && strings.Contains(item.Key, version) {
			filtred = append(filtred, item)
		}
	}
	return filtred
}

func getVersion(objStorageFile string) string {
	return strings.Split(strings.Split(objStorageFile, "@")[1], "/")[0]
}

func latestVersion(arr []minio.ObjectInfo) string {
	var versions []string
	for _, curr := range arr {
		versions = append(versions, getVersion(curr.Key))
	}
	semver.Sort(versions)
	return versions[len(versions)-1]
}

func getLatestVersion(objectStorageManager storage.ObjectStorageManager, pkg string, version string, fileName string) string {
	objs := objectStorageManager.ListObjects(fmt.Sprintf("%s@%s", pkg, version))
	filtred := filter(objs, fileName, version)

	if len(filtred) == 0 {
		return fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	}

	lversion := latestVersion(filtred)
	return fmt.Sprintf("%s@%s%s", pkg, lversion, fileName)
}

func GetFile(objectStorageManager storage.ObjectStorageManager, pkg string, version string, fileName string) (*minio.Object, *errors.GimmeError) {
	valid := semver.IsValid(fmt.Sprintf("v%v", version))
	fmt.Println(valid)
	if !valid {
		return nil, errors.NewError(errors.BadRequest, fmt.Errorf("invalid version"))
	}

	var objectPath string
	splited := strings.Split(version, ".")
	if len(splited) == 3 {
		objectPath = fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	} else {
		objectPath = getLatestVersion(objectStorageManager, pkg, version, fileName)
	}

	fmt.Println(objectPath)
	return objectStorageManager.GetObject(objectPath)
}
