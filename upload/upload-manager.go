package upload

import (
	"archive/zip"
	"errors"
	"fmt"
	"mime/multipart"
	"regexp"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/drouian-m/gimme/storage"
)

func ValidateFile(file *multipart.FileHeader) error {
	contentType := file.Header.Get("Content-Type")
	if len(contentType) == 0 || contentType != "application/zip" {
		return errors.New("Invalid input file type. (accepted types : application/zip)")
	}

	return nil
}

func ArchiveProcessor(name string, version string, objectStorageManager *storage.ObjectStorageManager, file *multipart.FileHeader) error {
	src, _ := file.Open()
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {
			logrus.Error("[ArchiveProcessor] Fail to close file")
		}
	}(src)

	//IDEA: did I need to also support single file import ?
	// we can detect file type here
	// - js / css case => upload directly in the object storage in a new folder <package>@<version>/<file>
	// - archive => unzip and upload each file in the object storage (same folder convention)

	archive, err := zip.NewReader(src, file.Size)
	if err != nil {
		return err
	}

	folderName := fmt.Sprintf("%s@%s", name, version)

	var re = regexp.MustCompile(`^[a-zA-Z0-9-_]+`)

	nbFiles := len(archive.File)
	if nbFiles == 0 {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(nbFiles)

	for i := 0; i < nbFiles; i++ {
		go func(i int) {
			defer wg.Done()
			currentFile := archive.File[i]
			logrus.Debug("Unzipping file ", currentFile.Name)
			fileName := re.ReplaceAllString(currentFile.FileHeader.Name, folderName)
			err := objectStorageManager.AddObject(fileName, currentFile)
			if err != nil {
				logrus.Errorf("Error while processing file %s", fileName)
			}
		}(i)
	}

	wg.Wait()
	return nil
}
