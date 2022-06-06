package archive_validator

import (
	"fmt"
	"mime/multipart"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/pkg/array"
	"github.com/sirupsen/logrus"
)

var validTypes = []string{"application/octet-stream", "application/zip"}

func ValidateFile(file *multipart.FileHeader) *errors.GimmeError {
	if file == nil {
		logrus.Errorf("[UploadManager] ValidateFile - Empty input file")
		return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("input file is required. (accepted types : application/zip)"))
	}

	contentType := file.Header.Get("Content-Type")
	if len(contentType) == 0 || !array.ArrayContains(validTypes, contentType) {
		logrus.Errorf("[UploadManager] ValidateFile - Invalid input file type. Content type : %s", contentType)
		return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("invalid input file type. (accepted types : application/zip)"))
	}

	return nil
}
