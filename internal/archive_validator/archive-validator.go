package archive_validator

import (
	"fmt"
	"mime"
	"mime/multipart"
	"slices"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/sirupsen/logrus"
)

var validTypes = []string{"application/octet-stream", "application/zip"}

func ValidateFile(file *multipart.FileHeader) *errors.GimmeError {
	if file == nil {
		logrus.Errorf("[ArchiveValidator] ValidateFile - Empty input file")
		return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("input file is required. (accepted types : application/zip)"))
	}

	contentType := file.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !slices.Contains(validTypes, mediaType) {
		logrus.Errorf("[ArchiveValidator] ValidateFile - Invalid input file type. Content type : %s", contentType)
		return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("invalid input file type. (accepted types : application/zip)"))
	}

	return nil
}
