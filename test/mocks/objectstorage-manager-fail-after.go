package mocks

import (
	"archive/zip"
	"context"
	"fmt"
	"sync/atomic"

	"github.com/ziggornif/gimme/internal/errors"

	"github.com/minio/minio-go/v7"
)

// OSManager mirrors storage.ObjectStorageManager instead of importing it:
// internal/storage's own tests live in package storage and import this package,
// so a reference to internal/storage here is an import cycle.
type OSManager interface {
	CreateBucket(ctx context.Context, bucketName string, location string) *errors.GimmeError
	AddObject(ctx context.Context, objectName string, file *zip.File, integrity string) *errors.GimmeError
	AddBytes(ctx context.Context, objectName string, data []byte, contentType string, integrity string) *errors.GimmeError
	GetObject(ctx context.Context, objectName string) (*minio.Object, *errors.GimmeError)
	ObjectExists(ctx context.Context, objectName string) bool
	ListObjects(ctx context.Context, objectParentName string) []minio.ObjectInfo
	ListObjectsPage(ctx context.Context, prefix string, after string, limit int) ([]minio.ObjectInfo, bool)
	ListCommonPrefixes(ctx context.Context, prefix string) []string
	RemoveObjects(ctx context.Context, objectParentName string) *errors.GimmeError
	Ping(ctx context.Context) *errors.GimmeError
}

// MockOSManagerFailAfter delegates every call to a real storage manager and
// fails the AddObject call numbered FailAt (1-based). MockOSManagerErr fails
// every call, which aborts before anything is stored — the half-written
// package this reproduces needs some entries to land first.
type MockOSManagerFailAfter struct {
	OSManager
	FailAt int64
	calls  atomic.Int64
}

func (osc *MockOSManagerFailAfter) AddObject(ctx context.Context, objectName string, file *zip.File, integrity string) *errors.GimmeError {
	if osc.calls.Add(1) == osc.FailAt {
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("boom"))
	}
	return osc.OSManager.AddObject(ctx, objectName, file, integrity)
}
