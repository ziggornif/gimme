package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGimmeError_String(t *testing.T) {
	err := NewError(InternalError, fmt.Errorf("boom"))
	assert.Equal(t, "boom", err.String())
}

func TestGimmeError_GetHTTPCode(t *testing.T) {
	err := NewError(InternalError, fmt.Errorf("boom"))
	assert.Equal(t, 500, err.GetHTTPCode())
}
