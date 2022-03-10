package upload

import (
	"archive/zip"
	"fmt"
	"io"
	"mime/multipart"
	"regexp"
	"sync"

	"github.com/gimme-cdn/gimme/internal/storage"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/gimme-cdn/gimme/pkg/array"

	"github.com/sirupsen/logrus"
)

var validTypes = []string{"application/octet-stream", "application/zip"}

func ValidateFile(file *multipart.FileHeader) *errors.GimmeError {
	if file == nil {
		logrus.Errorf("[UploadManager] ValidateFile - Empty input file")
		return errors.NewError(errors.BadRequest, fmt.Errorf("input file is required. (accepted types : application/zip)"))
	}

	contentType := file.Header.Get("Content-Type")
	if len(contentType) == 0 || !array.ArrayContains(validTypes, contentType) {
		logrus.Errorf("[UploadManager] ValidateFile - Invalid input file type. Content type : %s", contentType)
		return errors.NewError(errors.BadRequest, fmt.Errorf("invalid input file type. (accepted types : application/zip)"))
	}

	return nil
}

func ArchiveProcessor(name string, version string, objectStorageManager storage.ObjectStorageManager, file io.ReaderAt, fileSize int64) *errors.GimmeError {

	//IDEA: did I need to also support single file import ?
	// we can detect file type here
	// - js / css case => upload directly in the object storage in a new folder <package>@<version>/<file>
	// - archive => unzip and upload each file in the object storage (same folder convention)

	archive, err := zip.NewReader(file, fileSize)
	if err != nil {
		logrus.Error("[UploadManager] ArchiveProcessor - Error while reading zip file", err)
		return errors.NewError(errors.InternalError, fmt.Errorf("error while reading zip file"))
	}

	folderName := fmt.Sprintf("%s@%s", name, version)

	if exists := objectStorageManager.ObjectExists(folderName); exists {
		return errors.NewError(errors.Conflict, fmt.Errorf("the package %v already exists", folderName))
	}

	var re = regexp.MustCompile(`^[a-zA-Z0-9-_]+`)

	nbFiles := len(archive.File)

	var wg sync.WaitGroup
	wg.Add(nbFiles)

	for _, currentFile := range archive.File {
		go func(currentFile *zip.File) {
			defer wg.Done()
			logrus.Debug("[UploadManager] ArchiveProcessor - Unzipping file ", currentFile.Name)
			fileName := re.ReplaceAllString(currentFile.FileHeader.Name, folderName)
			err := objectStorageManager.AddObject(fileName, currentFile)
			if err != nil {
				logrus.Errorf("[UploadManager] ArchiveProcessor - Error while processing file %s", fileName)
			}
		}(currentFile)
	}

	wg.Wait()
	return nil
}
