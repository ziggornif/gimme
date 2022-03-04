package errors

import (
	"net/http"
)

type ErrorKindEnum string

const (
	Unauthorized  ErrorKindEnum = "Unauthorized"
	InternalError               = "InternalError"
	BadRequest                  = "BadRequest"
)

var httpCodes = map[ErrorKindEnum]int{
	Unauthorized:  http.StatusUnauthorized,
	InternalError: http.StatusInternalServerError,
	BadRequest:    http.StatusBadRequest,
}

type GimmeError struct {
	Kind  ErrorKindEnum
	Error error
}

func NewError(kind ErrorKindEnum, err error) *GimmeError {
	return &GimmeError{kind, err}
}

func (err GimmeError) String() string {
	return err.Error.Error()
}

func (err GimmeError) GetHTTPCode() int {
	return httpCodes[err.Kind]
}
