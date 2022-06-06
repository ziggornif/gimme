package mocks

import (
	"archive/zip"
	"strings"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManager struct{}

func (osc *MockOSManager) CreateBucket(_ string, _ string) *errors.GimmeError {
	return nil
}
func (osc *MockOSManager) AddObject(_ string, _ *zip.File) *errors.GimmeError {
	return nil
}
func (osc *MockOSManager) GetObject(_ string) (*minio.Object, *errors.GimmeError) {
	return &minio.Object{}, nil
}

func (osc *MockOSManager) ObjectExists(_ string) bool {
	return false
}

func (osc *MockOSManager) ListObjects(fileName string) []minio.ObjectInfo {
	var objs = []minio.ObjectInfo{{
		Key: "test@1.0.0",
	}, {
		Key: "test@1.0.0/test.js",
	}, {
		Key: "test@1.1.0",
	}, {
		Key: "test@1.1.0/test.js",
	}, {
		Key: "test@1.1.1",
	}, {
		Key: "test@1.1.1/test.js",
	}}

	if len(fileName) > 0 {
		var filtered []minio.ObjectInfo
		for _, item := range objs {
			if strings.Contains(item.Key, fileName) {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}

	return objs
}

func (osc *MockOSManager) RemoveObjects(_ string) *errors.GimmeError {
	return nil
}
