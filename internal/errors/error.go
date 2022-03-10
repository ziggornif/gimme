package errors

import (
	"net/http"
)

type ErrorKindEnum string

const (
	Unauthorized  ErrorKindEnum = "Unauthorized"
	InternalError               = "InternalError"
	BadRequest                  = "BadRequest"
	Conflict                    = "Conflict"
)

var httpCodes = map[ErrorKindEnum]int{
	BadRequest:    http.StatusBadRequest,
	Conflict:      http.StatusConflict,
	InternalError: http.StatusInternalServerError,
	Unauthorized:  http.StatusUnauthorized,
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
