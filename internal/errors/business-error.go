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

// Unwrap returns the wrapped error so that errors.Is and errors.As can
// traverse the error chain through a GimmeError.
func (e GimmeError) Unwrap() error {
	return e.Err
}

func (e GimmeError) GetHTTPCode() int {
	if code, ok := httpCodes[e.Kind]; ok {
		return code
	}
	return http.StatusInternalServerError
}
