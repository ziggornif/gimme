package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGimmeError_Error(t *testing.T) {
	err := NewBusinessError(InternalError, fmt.Errorf("boom"))
	assert.Equal(t, "boom", err.Error())
}

func TestGimmeError_GetHTTPCode(t *testing.T) {
	err := NewBusinessError(InternalError, fmt.Errorf("boom"))
	assert.Equal(t, 500, err.GetHTTPCode())
}

func TestGimmeError_GetHTTPCode_BadRequest(t *testing.T) {
	err := NewBusinessError(BadRequest, fmt.Errorf("bad"))
	assert.Equal(t, 400, err.GetHTTPCode())
}

func TestGimmeError_GetHTTPCode_Conflict(t *testing.T) {
	err := NewBusinessError(Conflict, fmt.Errorf("conflict"))
	assert.Equal(t, 409, err.GetHTTPCode())
}

func TestGimmeError_GetHTTPCode_Unauthorized(t *testing.T) {
	err := NewBusinessError(Unauthorized, fmt.Errorf("unauth"))
	assert.Equal(t, 401, err.GetHTTPCode())
}

func TestGimmeError_GetHTTPCode_NotImplemented(t *testing.T) {
	err := NewBusinessError(NotImplemented, fmt.Errorf("not implemented"))
	assert.Equal(t, 501, err.GetHTTPCode())
}

func TestGimmeError_GetHTTPCode_UnknownKind(t *testing.T) {
	// An unknown Kind must default to 500 (Internal Server Error) rather than
	// returning 0, which Go interprets as HTTP 200 OK.
	err := NewBusinessError(ErrorKindEnum("Unknown"), fmt.Errorf("unknown"))
	assert.Equal(t, 500, err.GetHTTPCode())
}

func TestGimmeError_Error_NilErr(t *testing.T) {
	err := NewBusinessError(BadRequest, nil)
	assert.Equal(t, "BadRequest", err.Error())
}
