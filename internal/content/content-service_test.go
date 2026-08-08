package content

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
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
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	assert.Nil(t, err)
}

func TestContentService_CreatePackageZipErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})

	fileName := "../../resources/tests/test.zip"
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, 1)
	assert.Equal(t, "error while reading zip file", err.Error())
}

func TestContentService_CreatePackageUploadErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerErr{}, nil, 0, UploadLimits{})

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	require.NotNil(t, err)
	assert.Equal(t, errors.ErrorKindEnum(errors.InternalError), err.Kind)
}

func TestContentService_CreatePackageExists(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerExists{}, nil, 0, UploadLimits{})

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	assert.Equal(t, "the package test@1.0.0 already exists", err.Error())
}

func TestContentService_GetFileSemverErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	_, err := service.GetFile(context.Background(), "test", "a.b.c", "test.js")
	assert.Equal(t, "invalid version (asked version must be semver compatible)", err.Error())
}

func TestContentService_GetFile(t *testing.T) {
	osm := &mocks.MockOSManager{}
	service := NewContentService(osm, nil, 0, UploadLimits{})
	file, err := service.GetFile(context.Background(), "test", "1.1.1", "/test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
	assert.Equal(t, "test@1.1.1/test.js", osm.LastGetObjectPath())
}

// Partial versions must be compared component-wise: 1.10.0 is newer than 1.9.9,
// and 10.0.0 is a different major that pkg@1 must never resolve to.
func TestContentService_GetMajorFile(t *testing.T) {
	osm := &mocks.MockOSManager{}
	service := NewContentService(osm, nil, 0, UploadLimits{})
	file, err := service.GetFile(context.Background(), "test", "1", "/test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
	assert.Equal(t, "test@1.10.0/test.js", osm.LastGetObjectPath())
}

func TestContentService_GetMinorFile(t *testing.T) {
	osm := &mocks.MockOSManager{}
	service := NewContentService(osm, nil, 0, UploadLimits{})
	file, err := service.GetFile(context.Background(), "test", "1.1", "/test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
	// 1.1 must not swallow 1.10.0 or 1.11.0.
	assert.Equal(t, "test@1.1.1/test.js", osm.LastGetObjectPath())
}

// A partial version must resolve to a stable release only: 2.0.0-rc.1 is not
// what pkg@2 promises.
func TestContentService_GetFile_PartialVersionSkipsPrerelease(t *testing.T) {
	osm := &mocks.MockOSManager{}
	service := NewContentService(osm, nil, 0, UploadLimits{})
	_, err := service.GetFile(context.Background(), "test", "2", "/test.js")
	assert.Nil(t, err)
	assert.Equal(t, "test@2/test.js", osm.LastGetObjectPath())
}

// A file name must match a whole path segment: test.js must not be satisfied by
// test.js.map, and asking for the map must reach the version that holds it.
func TestContentService_GetFile_FileNameIsNotASubstringMatch(t *testing.T) {
	osm := &mocks.MockOSManager{}
	service := NewContentService(osm, nil, 0, UploadLimits{})

	file, err := service.GetFile(context.Background(), "test", "1.11", "/test.js.map")
	assert.NotNil(t, file)
	assert.Nil(t, err)
	assert.Equal(t, "test@1.11.0/test.js.map", osm.LastGetObjectPath())

	// 1.11.0 holds no test.js, only test.js.map: nothing resolves, so the
	// unresolved partial version is queried and the storage answers 404.
	osm = &mocks.MockOSManager{}
	service = NewContentService(osm, nil, 0, UploadLimits{})
	_, err = service.GetFile(context.Background(), "test", "1.11", "/test.js")
	assert.Nil(t, err)
	assert.Equal(t, "test@1.11/test.js", osm.LastGetObjectPath())
}

func TestContentService_GetFiles(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	files, err := service.GetFiles(context.Background(), "test", "1.1.1")
	assert.Equal(t, 2, len(files))
	assert.Nil(t, err)
}

func TestContentService_DeletePackage(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Nil(t, err)
}

func TestContentService_DeletePackageErr(t *testing.T) {
	service := NewContentService(&mocks.MockOSManagerErr{}, nil, 0, UploadLimits{})
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Equal(t, "boom", err.Error())
}

func TestContentService_GetLatestVersionEmpty(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	result := service.getLatestVersion([]minio.ObjectInfo{})
	assert.Equal(t, "", result)
}

func TestContentService_GetVersion_NoAtSign(t *testing.T) {
	// Object keys without '@' must not panic and must return an empty string.
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	result := service.getVersion("malformed-object-key-without-at-sign")
	assert.Equal(t, "", result)
}

func TestContentService_GetLatestVersion_MalformedKeys(t *testing.T) {
	// Malformed entries (no '@') must be skipped; the valid entry wins.
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
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
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	file, err := service.GetFile(context.Background(), "test", "1.1", "test.js")
	assert.NotNil(t, file)
	assert.Nil(t, err)
}

func TestContentService_GetFile_CacheMiss_StoresEntry(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour, UploadLimits{})

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
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour, UploadLimits{})

	file, err := service.GetFile(context.Background(), "test", "1.1", "/test.js")
	require.Nil(t, err)
	assert.NotNil(t, file)
	// Cache hit → Get called once, Set never called
	assert.Equal(t, 1, cm.GetCalls)
	assert.Equal(t, 0, cm.SetCalls)
}

func TestContentService_GetFile_PinnedVersion_SkipsCache(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour, UploadLimits{})

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

	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour, UploadLimits{})
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
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	err := service.DeletePackage(context.Background(), "test", "1.1.1")
	assert.Nil(t, err)
}

// --- Metrics instrumentation tests ---

func TestContentService_CreatePackage_IncrementsUploadedCounter(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	before := testutil.ToFloat64(metrics.PackagesUploadedTotal)

	fileName := "../../test/test.zip"
	fi, _ := os.Stat(fileName)
	reader, _ := os.Open(fileName)
	err := service.CreatePackage(context.Background(), "metric-test", "1.0.0", reader, fi.Size())
	require.Nil(t, err)

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.PackagesUploadedTotal))
}

func TestContentService_DeletePackage_IncrementsDeletedCounter(t *testing.T) {
	service := NewContentService(&mocks.MockOSManager{}, nil, 0, UploadLimits{})
	before := testutil.ToFloat64(metrics.PackagesDeletedTotal)

	err := service.DeletePackage(context.Background(), "metric-test", "1.0.0")
	require.Nil(t, err)

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.PackagesDeletedTotal))
}

func TestContentService_GetFile_CacheHit_IncrementsHitCounter(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	cm.Seed("hit-pkg@1.1/file.js", &cache.CacheEntry{ObjectPath: "hit-pkg@1.1.1/file.js"})
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour, UploadLimits{})

	before := testutil.ToFloat64(metrics.CacheHitsTotal)

	_, err := service.GetFile(context.Background(), "hit-pkg", "1.1", "/file.js")
	require.Nil(t, err)

	assert.Equal(t, before+1, testutil.ToFloat64(metrics.CacheHitsTotal))
}

func TestContentService_GetFile_CacheMiss_IncrementsMissCounter(t *testing.T) {
	cm := mocks.NewMockCacheManager()
	service := NewContentService(&mocks.MockOSManager{}, cm, 1*time.Hour, UploadLimits{})

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

func buildArchiveWithContent(t *testing.T, content []byte) (*bytes.Reader, int64) {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	entry, err := writer.Create("root/large.bin")
	require.NoError(t, err)
	_, err = entry.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return bytes.NewReader(buf.Bytes()), int64(buf.Len())
}

func TestContentService_CreatePackage_UploadLimits(t *testing.T) {
	t.Run("entry count", func(t *testing.T) {
		manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
		service := NewContentService(manager, nil, 0, UploadLimits{MaxEntries: 2})
		reader, size := buildArchive(t, 3)

		err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)

		require.NotNil(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, err.GetHTTPCode())
		assert.Contains(t, err.Error(), "upload.max_entries")
		assert.Empty(t, manager.sortedKeys())
	})

	t.Run("uncompressed size", func(t *testing.T) {
		manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
		service := NewContentService(manager, nil, 0, UploadLimits{MaxUncompressedSize: 1024})
		reader, size := buildArchiveWithContent(t, make([]byte, 4096))

		err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)

		require.NotNil(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, err.GetHTTPCode())
		assert.Contains(t, err.Error(), "upload.max_uncompressed_size")
		assert.Empty(t, manager.sortedKeys())
	})

	t.Run("request size", func(t *testing.T) {
		manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
		service := NewContentService(manager, nil, 0, UploadLimits{MaxSize: 1})
		reader, size := buildArchive(t, 1)

		err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)

		require.NotNil(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, err.GetHTTPCode())
		assert.Contains(t, err.Error(), "upload.max_size")
		assert.Empty(t, manager.sortedKeys())
	})

	t.Run("under every limit", func(t *testing.T) {
		manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
		reader, size := buildArchive(t, 3)
		service := NewContentService(manager, nil, 0, UploadLimits{
			MaxSize:             size + 1,
			MaxEntries:          4,
			MaxUncompressedSize: 22,
		})

		err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)

		require.Nil(t, err)
		assert.Len(t, manager.sortedKeys(), 3)
	})

	t.Run("zero values disable checks", func(t *testing.T) {
		manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
		service := NewContentService(manager, nil, 0, UploadLimits{})
		reader, size := buildArchive(t, 3)

		err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)

		require.Nil(t, err)
		assert.Len(t, manager.sortedKeys(), 3)
	})
}

func TestContentService_CreatePackage_LimitsConcurrency(t *testing.T) {
	limit := runtime.NumCPU() * 4
	entries := limit + 50

	manager := &concurrencyTrackingOSManager{MockOSManager: &mocks.MockOSManager{}}
	service := NewContentService(manager, nil, 0, UploadLimits{})

	reader, size := buildArchive(t, entries)
	err := service.CreatePackage(context.Background(), "test", "1.0.0", reader, size)
	require.Nil(t, err)

	assert.Equal(t, int64(entries), manager.calls.Load(), "every archive entry must be uploaded")
	assert.LessOrEqual(t, manager.peak.Load(), int64(limit), "concurrent uploads must stay at or below the limit")
}

// --- ZIP entry key layout tests (#42, #43) ---

// recordingOSManager records every object key handed to AddObject.
type recordingOSManager struct {
	*mocks.MockOSManager
	mu   sync.Mutex
	keys []string
}

func (osc *recordingOSManager) AddObject(_ context.Context, objectName string, _ *zip.File) *errors.GimmeError {
	osc.mu.Lock()
	defer osc.mu.Unlock()
	osc.keys = append(osc.keys, objectName)
	return nil
}

func (osc *recordingOSManager) sortedKeys() []string {
	osc.mu.Lock()
	defer osc.mu.Unlock()
	out := append([]string(nil), osc.keys...)
	sort.Strings(out)
	return out
}

// buildArchiveFrom builds an in-memory zip archive holding the given entry names.
// A name ending with "/" is written as a directory entry.
func buildArchiveFrom(t *testing.T, names ...string) (*bytes.Reader, int64) {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if strings.HasSuffix(name, "/") {
			header.SetMode(fs.ModeDir | 0o755)
		}
		entry, err := writer.CreateHeader(header)
		require.NoError(t, err)
		if !strings.HasSuffix(name, "/") {
			_, err = entry.Write([]byte("content of " + name))
			require.NoError(t, err)
		}
	}
	require.NoError(t, writer.Close())

	return bytes.NewReader(buf.Bytes()), int64(buf.Len())
}

func TestContentService_CreatePackage_KeyLayout(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		expected []string
	}{
		{
			name:     "single root folder is stripped",
			entries:  []string{"awesome-lib/", "awesome-lib/app.js", "awesome-lib/style.css", "awesome-lib/img/logo.svg"},
			expected: []string{"pkg@1.0.0/app.js", "pkg@1.0.0/img/logo.svg", "pkg@1.0.0/style.css"},
		},
		{
			name:     "files at the archive root keep their own names",
			entries:  []string{"app.js", "style.css", "img/logo.svg"},
			expected: []string{"pkg@1.0.0/app.js", "pkg@1.0.0/img/logo.svg", "pkg@1.0.0/style.css"},
		},
		{
			name:     "several top level folders keep their internal paths",
			entries:  []string{"js/app.js", "vendor/app.js", "css/style.css"},
			expected: []string{"pkg@1.0.0/css/style.css", "pkg@1.0.0/js/app.js", "pkg@1.0.0/vendor/app.js"},
		},
		{
			name:     "nested folders under a single root",
			entries:  []string{"dist/js/app.js", "dist/js/vendor.js", "dist/css/style.css"},
			expected: []string{"pkg@1.0.0/css/style.css", "pkg@1.0.0/js/app.js", "pkg@1.0.0/js/vendor.js"},
		},
		{
			name:     "dot slash prefixes are normalised",
			entries:  []string{"./app.js", "./img/logo.svg"},
			expected: []string{"pkg@1.0.0/app.js", "pkg@1.0.0/img/logo.svg"},
		},
		{
			name:     "dot directories stay inside the package namespace",
			entries:  []string{".well-known/probe.txt", "app.js"},
			expected: []string{"pkg@1.0.0/.well-known/probe.txt", "pkg@1.0.0/app.js"},
		},
		{
			name:     "single file at the archive root",
			entries:  []string{"app.js"},
			expected: []string{"pkg@1.0.0/app.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
			service := NewContentService(manager, nil, 0, UploadLimits{})

			reader, size := buildArchiveFrom(t, tt.entries...)
			err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)
			require.Nil(t, err)
			assert.Equal(t, tt.expected, manager.sortedKeys())

			// Same archive uploaded again must produce byte-identical keys.
			second := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
			secondService := NewContentService(second, nil, 0, UploadLimits{})
			reader2, size2 := buildArchiveFrom(t, tt.entries...)
			err = secondService.CreatePackage(context.Background(), "pkg", "1.0.0", reader2, size2)
			require.Nil(t, err)
			assert.Equal(t, manager.sortedKeys(), second.sortedKeys())
		})
	}
}

func TestContentService_CreatePackage_RootDetection(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		expected []string
	}{
		{
			name:     "a metadata file at the root does not block the wrapper folder",
			entries:  []string{"dist/app.js", "dist/css/style.css", "README.md"},
			expected: []string{"pkg@1.0.0/README.md", "pkg@1.0.0/app.js", "pkg@1.0.0/css/style.css"},
		},
		{
			name:     "metadata names are matched case insensitively",
			entries:  []string{"dist/app.js", "readme.md", "LICENSE", "Changelog.md", ".gitignore"},
			expected: []string{"pkg@1.0.0/.gitignore", "pkg@1.0.0/Changelog.md", "pkg@1.0.0/LICENSE", "pkg@1.0.0/app.js", "pkg@1.0.0/readme.md"},
		},
		{
			name:     "a content file at the root still blocks the wrapper folder",
			entries:  []string{"app.js", "style.css", "img/logo.svg"},
			expected: []string{"pkg@1.0.0/app.js", "pkg@1.0.0/img/logo.svg", "pkg@1.0.0/style.css"},
		},
		{
			name:     "two top level folders are never stripped",
			entries:  []string{"src/a.js", "docs/b.md", "README.md"},
			expected: []string{"pkg@1.0.0/README.md", "pkg@1.0.0/docs/b.md", "pkg@1.0.0/src/a.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
			service := NewContentService(manager, nil, 0, UploadLimits{})

			reader, size := buildArchiveFrom(t, tt.entries...)
			err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)
			require.Nil(t, err)
			assert.Equal(t, tt.expected, manager.sortedKeys())
		})
	}
}

func TestContentService_CreatePackage_DropsArchiveJunk(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		expected []string
	}{
		{
			name:     "a Finder made archive is stripped and its metadata dropped",
			entries:  []string{"dist/app.js", "__MACOSX/dist/._app.js", ".DS_Store", "dist/.DS_Store"},
			expected: []string{"pkg@1.0.0/app.js"},
		},
		{
			name:     "DS_Store is dropped at every level",
			entries:  []string{"app.js", ".DS_Store", "img/.DS_Store", "img/css/.DS_Store", "img/logo.svg"},
			expected: []string{"pkg@1.0.0/app.js", "pkg@1.0.0/img/logo.svg"},
		},
		{
			name:     "AppleDouble files are dropped outside __MACOSX too",
			entries:  []string{"dist/app.js", "dist/._app.js"},
			expected: []string{"pkg@1.0.0/app.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
			service := NewContentService(manager, nil, 0, UploadLimits{})

			reader, size := buildArchiveFrom(t, tt.entries...)
			err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)
			require.Nil(t, err)
			assert.Equal(t, tt.expected, manager.sortedKeys())
		})
	}
}

func TestContentService_CreatePackage_RejectsJunkOnlyArchive(t *testing.T) {
	manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
	service := NewContentService(manager, nil, 0, UploadLimits{})

	reader, size := buildArchiveFrom(t, ".DS_Store", "__MACOSX/._x")
	err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)

	require.NotNil(t, err, "an archive holding nothing but junk must be rejected")
	assert.Equal(t, errors.ErrorKindEnum(errors.BadRequest), err.Kind)
	assert.Empty(t, manager.sortedKeys())
}

// Escaped rather than written literally: an editor or tool normalising this
// source file would make the two constants byte-identical and the tests below
// vacuous.
const (
	nfcCafe = "caf\u00e9.js"  // precomposed e-acute, the form a browser sends
	nfdCafe = "cafe\u0301.js" // e + combining acute, the form macOS stores
)

func TestContentService_NFCFixturesDiffer(t *testing.T) {
	require.NotEqual(t, nfcCafe, nfdCafe, "the NFC and NFD fixtures must not be byte-identical")
	require.Len(t, nfcCafe, 8)
	require.Len(t, nfdCafe, 9)
}

func TestContentService_CreatePackage_NormalisesEntryNamesToNFC(t *testing.T) {
	manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
	service := NewContentService(manager, nil, 0, UploadLimits{})

	reader, size := buildArchiveFrom(t, "app.js", nfdCafe)
	err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)

	require.Nil(t, err)
	assert.Equal(t, []string{"pkg@1.0.0/app.js", "pkg@1.0.0/" + nfcCafe}, manager.sortedKeys(),
		"an NFD entry name must be stored under its NFC key, the form a browser requests")
}

func TestContentService_CreatePackage_RejectsDuplicatesAfterNormalisation(t *testing.T) {
	manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
	service := NewContentService(manager, nil, 0, UploadLimits{})

	reader, size := buildArchiveFrom(t, nfcCafe, nfdCafe)
	err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)

	require.NotNil(t, err, "entries colliding only after NFC normalisation must be rejected")
	assert.Equal(t, errors.ErrorKindEnum(errors.BadRequest), err.Kind)
	assert.Contains(t, err.Error(), `"`+nfcCafe+`"`, "the error must name the first colliding entry")
	assert.Contains(t, err.Error(), `"`+nfdCafe+`"`, "the error must name the second colliding entry")
	assert.Empty(t, manager.sortedKeys(), "nothing must be uploaded when the archive is rejected")
}

func TestContentService_CreatePackage_RejectsEscapingEntries(t *testing.T) {
	tests := []string{
		"../x.js",
		"../../evil.js",
		"/abs.js",
		"a/../../b.js",
		"./../x.js",
		"",
	}

	for _, entry := range tests {
		t.Run(fmt.Sprintf("entry %q", entry), func(t *testing.T) {
			manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
			service := NewContentService(manager, nil, 0, UploadLimits{})

			reader, size := buildArchiveFrom(t, "app.js", entry)
			err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)

			require.NotNil(t, err, "archive must be rejected")
			assert.Equal(t, errors.ErrorKindEnum(errors.BadRequest), err.Kind)
			assert.Empty(t, manager.sortedKeys(), "nothing must be uploaded when the archive is rejected")
		})
	}
}

func TestContentService_CreatePackage_RejectsDuplicateKeys(t *testing.T) {
	manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
	service := NewContentService(manager, nil, 0, UploadLimits{})

	reader, size := buildArchiveFrom(t, "js/app.js", "./js/app.js")
	err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)

	require.NotNil(t, err, "colliding entries must be rejected, never silently merged")
	assert.Equal(t, errors.ErrorKindEnum(errors.BadRequest), err.Kind)
	// Quote the names so neither assertion is satisfied by the other entry:
	// "js/app.js" is a substring of "./js/app.js" without the quotes.
	assert.Contains(t, err.Error(), `"js/app.js"`, "the error must name the first colliding entry")
	assert.Contains(t, err.Error(), `"./js/app.js"`, "the error must name the second colliding entry")
	assert.Contains(t, err.Error(), `"pkg@1.0.0/app.js"`, "the error must name the key they collide on")
	assert.Empty(t, manager.sortedKeys(), "nothing must be uploaded when the archive is rejected")
}

func TestContentService_CreatePackage_RejectsEmptyArchive(t *testing.T) {
	manager := &recordingOSManager{MockOSManager: &mocks.MockOSManager{}}
	service := NewContentService(manager, nil, 0, UploadLimits{})

	reader, size := buildArchiveFrom(t)
	err := service.CreatePackage(context.Background(), "pkg", "1.0.0", reader, size)

	require.NotNil(t, err, "an archive with no file must be rejected")
	assert.Equal(t, errors.ErrorKindEnum(errors.BadRequest), err.Kind)
}
