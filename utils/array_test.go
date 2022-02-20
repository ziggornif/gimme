package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArrayContains(t *testing.T) {
	testarr := []string{"foo", "bar", "baz"}
	assert.Equal(t, true, ArrayContains(testarr, "foo"))
}

func TestNotArrayContains(t *testing.T) {
	testarr := []string{"foo", "bar"}
	assert.Equal(t, false, ArrayContains(testarr, "baz"))
}
