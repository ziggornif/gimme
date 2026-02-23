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
