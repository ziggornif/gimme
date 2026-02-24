package errors

import (
	"net/http"
)

type ErrorKindEnum string

const (
	Unauthorized   ErrorKindEnum = "Unauthorized"
	InternalError  ErrorKindEnum = "InternalError"
	BadRequest     ErrorKindEnum = "BadRequest"
	Conflict       ErrorKindEnum = "Conflict"
	NotImplemented ErrorKindEnum = "NotImplemented"
)

var httpCodes = map[ErrorKindEnum]int{
	BadRequest:     http.StatusBadRequest,
	Conflict:       http.StatusConflict,
	InternalError:  http.StatusInternalServerError,
	Unauthorized:   http.StatusUnauthorized,
	NotImplemented: http.StatusNotImplemented,
}

type GimmeError struct {
	Kind ErrorKindEnum
	Err  error
}

func NewBusinessError(kind ErrorKindEnum, err error) *GimmeError {
	return &GimmeError{kind, err}
}

func (e GimmeError) Error() string {
	if e.Err == nil {
		return string(e.Kind)
	}
	return e.Err.Error()
}

func (e GimmeError) GetHTTPCode() int {
	return httpCodes[e.Kind]
}
