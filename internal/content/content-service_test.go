package content

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ziggornif/gimme/internal/cache"
	"github.com/ziggornif/gimme/internal/errors"
	"github.com/ziggornif/gimme/internal/metrics"
	"github.com/ziggornif/gimme/test/mocks"
)

func TestContentService_CreatePackage(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	assert.Nil(t, err)
}

func TestContentService_CreatePackageZipErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)

	fileName := "../../resources/tests/test.zip"
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, 1)
	assert.Equal(t, "error while reading zip file", err.Error())
}

func TestContentService_CreatePackageUploadErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerErr{}, nil, 0)

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	require.NotNil(t, err)
	assert.Equal(t, errors.ErrorKindEnum(errors.InternalError), err.Kind)
}

func TestContentService_CreatePackageExists(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerExists{}, nil, 0)

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	assert.Equal(t, "the package test@1.0.0 already exists", err.Error())
}

func TestContentService_GetFileSemverErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	_, err := service.GetFile(context.Background(), "test", "a.b.c", "test.js")
	assert.Equal(t, "invalid version (asked version must be semver compatible)", err.Error())
}

func TestContentService_GetFile(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	file, err := service.GetFile(context.Background(), "test", "1.1.1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetMajorFile(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	file, err := service.GetFile(context.Background(), "test", "1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetMinorFile(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	file, err := service.GetFile(context.Background(), "test", "1.1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetFiles(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	files, err := service.GetFiles(context.Background(), "test", "1.1.1")
	assert.Equal(t, 2, len(files))
	assert.Nil(t, err)
}

func TestContentService_DeletePackage(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Nil(t, err)
}

func TestContentService_DeletePackageErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerErr{}, nil, 0)
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Equal(t, "boom", err.Error())
}

func TestContentService_GetLatestVersionEmpty(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	result := service.getLatestVersion([]minio.ObjectInfo{})
	assert.Equal(t, "", result)
}

func TestContentService_GetVersion_NoAtSign(t *testing.T) {
	// Object keys without '@' must not panic and must return an empty string.
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	result := service.getVersion("malformed-object-key-without-at-sign")
	assert.Equal(t, "", result)
}

func TestContentService_GetLatestVersion_MalformedKeys(t *testing.T) {
	// Malformed entries (no '@') must be skipped; the valid entry wins.
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	objs := []minio.ObjectInfo{
		{Key: "malformed-no-at-sign"},
		{Key: "pkg@1.0.0/file.js"},
	}
	result := service.getLatestVersion(objs)
	assert.Equal(t, "1.0.0", result)
}

// --- Cache tests ---

func TestContentService_GetFile_CacheDisabled(t *testing.T) {
	// nil cache manager — behaviour identical to before
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	file, err := service.GetFile(context.Background(), "test", "1.1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetFile_CacheMiss_StoresEntry(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour)

	file, err := service.GetFile(context.Background(), "test", "1.1", "/test.js")
	require.Nil(t, err)
	assert.NotNil(t, file)
	// Cache miss → Get called once, Set called once
	assert.Equal(t, 1, cm.GetCalls)
	assert.Equal(t, 1, cm.SetCalls)
}

func TestContentService_GetFile_CacheHit_SkipsResolution(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	cm.Seed("test@1.1/test.js", &cache.CacheEntry{
		ObjectPath: "test@1.1.1/test.js",
	})
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour)

	file, err := service.GetFile(context.Background(), "test", "1.1", "/test.js")
	require.Nil(t, err)
	assert.NotNil(t, file)
	// Cache hit → Get called once, Set never called
	assert.Equal(t, 1, cm.GetCalls)
	assert.Equal(t, 0, cm.SetCalls)
}

func TestContentService_GetFile_PinnedVersion_SkipsCache(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour)

	// Pinned version (1.1.1) — cache must not be consulted or populated
	file, err := service.GetFile(context.Background(), "test", "1.1.1", "/test.js")
	require.Nil(t, err)
	assert.NotNil(t, file)
	assert.Equal(t, 0, cm.GetCalls)
	assert.Equal(t, 0, cm.SetCalls)
}

func TestContentService_DeletePackage_InvalidatesCache(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	cm.Seed("test@1.1.1/file.js", &cache.CacheEntry{ObjectPath: "test@1.1.1/file.js"})
	cm.Seed("test@1.1.1/file.css", &cache.CacheEntry{ObjectPath: "test@1.1.1/file.css"})
	// Partial version entries that may have resolved to 1.1.1
	cm.Seed("test@1.1/file.js", &cache.CacheEntry{ObjectPath: "test@1.1.1/file.js"})
	cm.Seed("test@1/file.js", &cache.CacheEntry{ObjectPath: "test@1.1.1/file.js"})

	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour)
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	require.Nil(t, err)
	// 1 exact prefix (test@1.1.1) + 2 partial prefixes (test@1.1, test@1)
	assert.Equal(t, 3, cm.DeleteByPrefixCalls)
	// Exact version entries must be gone
	_, ok1 := cm.Get(context.Background(), "test@1.1.1/file.js")
	_, ok2 := cm.Get(context.Background(), "test@1.1.1/file.css")
	assert.False(t, ok1)
	assert.False(t, ok2)
	// Partial version entries must also be gone
	_, ok3 := cm.Get(context.Background(), "test@1.1/file.js")
	_, ok4 := cm.Get(context.Background(), "test@1/file.js")
	assert.False(t, ok3)
	assert.False(t, ok4)
}

func TestContentService_DeletePackage_NoCacheManager(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Nil(t, err)
}

// --- Metrics instrumentation tests ---

func TestContentService_CreatePackage_IncrementsUploadedCounter(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	before := testutil.ToFloat64(metrics.PackagesUploadedTotal)

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "metric-test", "1.0.0", reader, fi.Size())
	require.Nil(t, err)

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.PackagesUploadedTotal))
}

func TestContentService_DeletePackage_IncrementsDeletedCounter(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0)
	before := testutil.ToFloat64(metrics.PackagesDeletedTotal)

	err := service.DeletePackage(context.Background(), "metric-test", "1.0.0")
	require.Nil(t, err)

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.PackagesDeletedTotal))
}

func TestContentService_GetFile_CacheHit_IncrementsHitCounter(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	cm.Seed("hit-pkg@1.1/file.js", &cache.CacheEntry{ObjectPath: "hit-pkg@1.1.1/file.js"})
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour)

	before := testutil.ToFloat64(metrics.CacheHitsTotal)

	_, err := service.GetFile(context.Background(), "hit-pkg", "1.1", "/file.js")
	require.Nil(t, err)

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.CacheHitsTotal))
}

func TestContentService_GetFile_CacheMiss_IncrementsMissCounter(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour)

	before := testutil.ToFloat64(metrics.CacheMissesTotal)

	_, err := service.GetFile(context.Background(), "miss-pkg", "1.1", "/file.js")
	require.Nil(t, err)

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.CacheMissesTotal))
}

func TestIsPinnedVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"1.0.0", true},
		{"1.0.1", true},
		{"0.0.1", true},
		{"1.0", false},
		{"1", false},
		{"1.0.0-rc.1", false},
		{"1.0.0+build.1", true},
		{"notasemver", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsPinnedVersion(tt.version))
		})
	}
}

// --- Upload concurrency tests ---

// concurrencyTrackingOSManager records the peak number of AddObject calls in
// flight at any moment, plus the total number of calls.
type concurrencyTrackingOSManager struct {
	*mocks.MockOSManager
	inFlight atomic.Int64
	peak     atomic.Int64
	calls    atomic.Int64
}

func (osc *concurrencyTrackingOSManager) AddObject(_ context.Context, _ string, _ *zip.File) *errors.GimmeError {
	osc.calls.Add(1)
	current := osc.inFlight.Add(1)
	for {
		peak := osc.peak.Load()
		if current <= peak || osc.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	// Hold the slot long enough for every concurrent upload to overlap.
	time.Sleep(10 * time.Millisecond)
	osc.inFlight.Add(-1)
	return nil
}

// buildArchive builds an in-memory zip archive holding entries files.
func buildArchive(t *testing.T, entries int) (*bytes.Reader, int64) {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for i := range entries {
		entry, err := writer.Create(fmt.Sprintf("root/file-%d.js", i))
		require.NoError(t, err)
		_, err = entry.Write([]byte("content"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	return bytes.NewReader(buf.Bytes()), int64(buf.Len())
}

func TestContentService_CreatePackage_LimitsConcurrency(t *testing.T) {
	limit := runtime.NumCPU() * 4
	entries := limit + 50

	manager := &concurrencyTrackingOSManager{MockOSManager: &mocks.MockOSManager{}}
	service := NewContentService(manager, nil, 0)

	reader, size := buildArchive(t, entries)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	require.Nil(t, err)

	assert.Equal(t, int64(entries), manager.calls.Load(), "every archive entry must be uploaded")
	assert.LessOrEqual(t, manager.peak.Load(), int64(limit), "concurrent uploads must stay at or below the limit")
}
