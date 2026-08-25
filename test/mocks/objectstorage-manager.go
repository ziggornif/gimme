package mocks

import (
	"archive/zip"
	"context"
	"strings"
	"sync"

	"github.com/ziggornif/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

type MockOSManager struct {
	// GetObjectPaths records every key GetObject was asked for, so tests can
	// assert on the path version resolution produced — not merely that some
	// object came back.
	GetObjectPaths       []string
	mu                   sync.Mutex
	AddObjectKeys        []string
	AddObjectIntegrities []string
	AddBytesKeys         []string
	AddBytesSizes        []int
	AddBytesIntegrities  []string
}

// LastGetObjectPath returns the last key passed to GetObject, or "" if it was never called.
func (osc *MockOSManager) LastGetObjectPath() string {
	if len(osc.GetObjectPaths) == 0 {
		return ""
	}
	return osc.GetObjectPaths[len(osc.GetObjectPaths)-1]
}

func (osc *MockOSManager) CreateBucket(_ context.Context, _ string, _ string) *errors.GimmeError {
	return nil
}

func (osc *MockOSManager) AddObject(_ context.Context, objectName string, _ *zip.File, integrity string) *errors.GimmeError {
	osc.mu.Lock()
	defer osc.mu.Unlock()
	osc.AddObjectKeys = append(osc.AddObjectKeys, objectName)
	osc.AddObjectIntegrities = append(osc.AddObjectIntegrities, integrity)
	return nil
}
func (osc *MockOSManager) AddBytes(_ context.Context, objectName string, data []byte, _ string, integrity string) *errors.GimmeError {
	osc.mu.Lock()
	defer osc.mu.Unlock()
	osc.AddBytesKeys = append(osc.AddBytesKeys, objectName)
	osc.AddBytesSizes = append(osc.AddBytesSizes, len(data))
	osc.AddBytesIntegrities = append(osc.AddBytesIntegrities, integrity)
	return nil
}
func (osc *MockOSManager) GetObject(_ context.Context, objectPath string) (*minio.Object, *errors.GimmeError) {
	osc.GetObjectPaths = append(osc.GetObjectPaths, objectPath)
	return &minio.Object{}, nil
}

func (osc *MockOSManager) ObjectExists(_ context.Context, _ string) bool {
	return false
}

func (osc *MockOSManager) ListObjects(_ context.Context, fileName string) []minio.ObjectInfo {
	// Two-digit components (1.10.0, 1.11.0) and a second major (10.0.0) are
	// required: without them no fixture can express the pkg@1 -> 10.0.0 bug.
	// 1.11.0 deliberately ships only a source map, so a request for test.js
	// must not resolve to it.
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
	}, {
		Key: "test@1.9.9",
	}, {
		Key: "test@1.9.9/test.js",
	}, {
		Key: "test@1.10.0",
	}, {
		Key: "test@1.10.0/test.js",
	}, {
		Key: "test@1.11.0",
	}, {
		Key: "test@1.11.0/test.js.map",
	}, {
		Key: "test@10.0.0",
	}, {
		Key: "test@10.0.0/test.js",
	}, {
		// Only pre-release in the 2.x line: pkg@2 must not resolve to it.
		Key: "test@2.0.0-rc.1",
	}, {
		Key: "test@2.0.0-rc.1/test.js",
	}}

	// Object storages list by prefix, not by substring.
	if len(fileName) > 0 {
		var filtered []minio.ObjectInfo
		for _, item := range objs {
			if strings.HasPrefix(item.Key, fileName) {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}

	return objs
}

func (osc *MockOSManager) RemoveObjects(_ context.Context, _ string) *errors.GimmeError {
	return nil
}

func (osc *MockOSManager) Ping(_ context.Context) *errors.GimmeError {
	return nil
}
