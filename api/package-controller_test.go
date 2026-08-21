package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ziggornif/gimme/configs"
	"github.com/ziggornif/gimme/internal/auth"
	"github.com/ziggornif/gimme/internal/content"
	"github.com/ziggornif/gimme/internal/storage"
	"github.com/ziggornif/gimme/test/mocks"
	"github.com/ziggornif/gimme/test/utils"
)

// newPackageTestStore creates a FileTokenStore in a temp dir for controller tests.
func newPackageTestStore(t *testing.T) *auth.FileTokenStore {
	t.Helper()
	store, err := auth.NewFileTokenStore("this-is-a-32-byte-secret-for-test", filepath.Join(t.TempDir(), "tokens.enc"))
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func initObjectStorage() storage.ObjectStorageManager {
	bucketName := envOrDefault("TEST_S3_BUCKET", "gimme")
	location := envOrDefault("TEST_S3_LOCATION", "eu-west-1")
	client, err := storage.NewObjectStorageClient(&configs.Configuration{
		S3Url:        envOrDefault("TEST_S3_URL", "localhost:9000"),
		S3Key:        envOrDefault("TEST_S3_KEY", "minioadmin"),
		S3Secret:     envOrDefault("TEST_S3_SECRET", "minioadmin"),
		S3BucketName: bucketName,
		S3Location:   location,
		S3SSL:        false,
	})
	if err != nil {
		panic(err.Error())
	}
	objectStorageManager := storage.NewObjectStorageManager(client)
	if err := objectStorageManager.CreateBucket(context.Background(), bucketName, location); err != nil {
		panic(err.Error())
	}
	return objectStorageManager
}

func createPackage(t *testing.T, router http.Handler, name string, version string, filePath string, token string) *httptest.ResponseRecorder {
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	file, _ := os.Open(filePath)
	defer func(file *os.File) {
		err := file.Close()
		assert.Nil(t, err)
	}(file)

	formFile,
		_ := writer.CreateFormFile("file", filepath.Base(filePath))
	_, err := io.Copy(formFile, file)
	assert.Nil(t, err)
	err = writer.WriteField("name", name)
	assert.Nil(t, err)
	err = writer.WriteField("version", version)
	assert.Nil(t, err)
	err = writer.Close()
	assert.Nil(t, err)

	return utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{
			Key: "Content-Type", Value: writer.FormDataContentType(),
		})
}

func TestPackageControllerGETErr(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager(newPackageTestStore(t))
	mockOSManager := mocks.MockOSManagerErr{}
	service := content.NewContentService(&mockOSManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0/file.js", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestPackageControllerNotFoundURL(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager(newPackageTestStore(t))
	mockOSManager := mocks.MockOSManagerErr{}
	service := content.NewContentService(&mockOSManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// countingReader counts the bytes actually consumed from a request body.
type countingReader struct {
	reader io.Reader
	read   int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.read += int64(n)
	return n, err
}

func TestPackageControllerCreatePackageRequestTooLarge(t *testing.T) {
	const maxSize = 300
	const payloadSize = 1 << 20

	router := gin.New()
	authManager := auth.NewAuthManager(newPackageTestStore(t))
	_, rawToken, tokenErr := authManager.CreateToken(context.Background(), "upload-test", "")
	require.Nil(t, tokenErr)
	mockOSManager := mocks.MockOSManager{}
	service := content.NewContentService(&mockOSManager, nil, 0, content.UploadLimits{MaxSize: maxSize})
	NewPackageController(router, authManager, service)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	formFile, err := writer.CreateFormFile("file", "package.zip")
	require.NoError(t, err)
	_, err = formFile.Write(make([]byte, payloadSize))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("name", "test"))
	require.NoError(t, writer.WriteField("version", "1.0.0"))
	require.NoError(t, writer.Close())
	require.Greater(t, payload.Len(), payloadSize)

	body := &countingReader{reader: bytes.NewReader(payload.Bytes())}
	response := utils.PerformRequest(router, "POST", "/packages", body,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)},
		utils.Header{Key: "Content-Type", Value: writer.FormDataContentType()})

	assert.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	assert.Contains(t, response.Body.String(), "upload.max_size")
	assert.LessOrEqual(t, body.read, int64(maxSize+1),
		"the body must stop being read at the limit, not be drained and then rejected")
}

func newUploadRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	router := gin.New()
	authManager := auth.NewAuthManager(newPackageTestStore(t))
	_, rawToken, tokenErr := authManager.CreateToken(context.Background(), "upload-test", "")
	require.Nil(t, tokenErr)
	mockOSManager := mocks.MockOSManager{}
	service := content.NewContentService(&mockOSManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)
	return router, rawToken
}

func uploadPayload(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	formFile, err := writer.CreateFormFile("file", "package.zip")
	require.NoError(t, err)
	_, err = formFile.Write(make([]byte, 1024))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("name", "test"))
	require.NoError(t, writer.WriteField("version", "1.0.0"))
	require.NoError(t, writer.Close())
	return payload, writer.FormDataContentType()
}

func TestPackageControllerCreatePackageMissingFileField(t *testing.T) {
	router, token := newUploadRouter(t)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	require.NoError(t, writer.WriteField("name", "test"))
	require.NoError(t, writer.WriteField("version", "1.0.0"))
	require.NoError(t, writer.Close())

	response := utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{Key: "Content-Type", Value: writer.FormDataContentType()})

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "input file is required. (accepted types : application/zip)")
}

func TestPackageControllerCreatePackageMalformedMultipart(t *testing.T) {
	router, token := newUploadRouter(t)
	payload, _ := uploadPayload(t)

	response := utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{Key: "Content-Type", Value: "multipart/form-data; boundary=WRONGBOUNDARY"})

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "multipart")
	assert.NotContains(t, response.Body.String(), "input file is required",
		"a client that did send a file must not be told it did not")
}

func TestPackageControllerCreatePackageNotMultipart(t *testing.T) {
	router, token := newUploadRouter(t)

	response := utils.PerformRequest(router, "POST", "/packages", bytes.NewReader([]byte(`{"name":"test"}`)),
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{Key: "Content-Type", Value: "application/json"})

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "multipart/form-data")
	assert.NotContains(t, response.Body.String(), "input file is required")
}

func TestPackageControllerCreatePackageSpillWriteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a read-only directory does not constrain root")
	}

	router, token := newUploadRouter(t)
	router.MaxMultipartMemory = 1
	tmpDir := t.TempDir()
	require.NoError(t, os.Chmod(tmpDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(tmpDir, 0o700) })
	t.Setenv("TMPDIR", tmpDir)

	payload, contentType := uploadPayload(t)
	response := utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{Key: "Content-Type", Value: contentType})

	assert.Equal(t, http.StatusInternalServerError, response.Code,
		"a failure writing the form to disk is a server fault, not a client error")
	assert.NotContains(t, response.Body.String(), "input file is required")
	assert.NotContains(t, response.Body.String(), tmpDir, "the response must not leak a server path")
}

func TestGetSlice_EmptyName(t *testing.T) {
	ctrl := &PackageController{}
	_, err := ctrl.getSlice("@1.0.0")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.GetHTTPCode())
	assert.Contains(t, err.Error(), "package name must not be empty")
}

func TestGetSlice_EmptyVersion(t *testing.T) {
	ctrl := &PackageController{}
	_, err := ctrl.getSlice("pkg@")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.GetHTTPCode())
	assert.Contains(t, err.Error(), "package version must not be empty")
}

func TestGetSlice_NoAtSign(t *testing.T) {
	ctrl := &PackageController{}
	_, err := ctrl.getSlice("pkg-without-at")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.GetHTTPCode())
}

func TestGetSlice_Valid(t *testing.T) {
	ctrl := &PackageController{}
	slice, err := ctrl.getSlice("mypkg@1.2.3")
	assert.Nil(t, err)
	assert.Equal(t, "mypkg", slice.Name)
	assert.Equal(t, "1.2.3", slice.Version)
}

func TestCacheControlHeader(t *testing.T) {
	tests := []struct {
		version  string
		expected string
	}{
		{"1.0.0", "public, max-age=31536000, immutable"},
		{"1.0.1", "public, max-age=31536000, immutable"},
		{"1.0", "public, max-age=300"},
		{"1", "public, max-age=300"},
		{"1.0.0-rc.1", "public, max-age=300"},
		{"1.0.0+build.1", "public, max-age=31536000, immutable"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.expected, cacheControlHeader(tt.version))
		})
	}
}

func TestETagMatches(t *testing.T) {
	tests := []struct {
		name        string
		ifNoneMatch string
		etag        string
		expected    bool
	}{
		{"exact match", `"abc"`, `"abc"`, true},
		{"weak request ETag", `W/"abc"`, `"abc"`, true},
		{"weak response ETag", `"abc"`, `W/"abc"`, true},
		{"match in second position", `"stale", W/"abc"`, `"abc"`, true},
		{"wildcard", `*`, `"abc"`, true},
		{"stale value", `"stale"`, `"abc"`, false},
		{"empty header", ``, `"abc"`, false},
		{"empty ETag", `*`, ``, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, etagMatches(tt.ifNoneMatch, tt.etag))
		})
	}
}

func TestParseAcceptEncoding(t *testing.T) {
	tests := []struct {
		header   string
		expected []content.Encoding
	}{
		{"", nil},
		{"br", []content.Encoding{content.EncodingBrotli}},
		{"gzip", []content.Encoding{content.EncodingGzip}},
		{"br, gzip", []content.Encoding{content.EncodingBrotli, content.EncodingGzip}},
		{"br;q=0.5, gzip;q=0.9", []content.Encoding{content.EncodingGzip, content.EncodingBrotli}},
		{"gzip;q=0", nil},
		{"*", []content.Encoding{content.EncodingBrotli, content.EncodingGzip}},
		{"deflate, zstd", nil},
		{"br;q=nope, gzip", []content.Encoding{content.EncodingGzip}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, parseAcceptEncoding(tt.header), tt.header)
	}
}

func TestNotModified(t *testing.T) {
	lastModified := time.Date(2026, time.August, 21, 12, 30, 45, 0, time.UTC)
	tests := []struct {
		name            string
		ifNoneMatch     string
		ifModifiedSince string
		lastModified    time.Time
		etag            string
		expected        bool
	}{
		{"matching If-None-Match", `"abc"`, "", lastModified, `"abc"`, true},
		{"stale If-None-Match takes precedence", `"stale"`, lastModified.Format(http.TimeFormat), lastModified, `"abc"`, false},
		{"If-Modified-Since exact match", "", lastModified.Format(http.TimeFormat), lastModified, `"abc"`, true},
		{"If-Modified-Since older", "", lastModified.Add(-time.Second).Format(http.TimeFormat), lastModified, `"abc"`, false},
		{"malformed date", "", "not-a-date", lastModified, `"abc"`, false},
		{"zero last modified", "", lastModified.Format(http.TimeFormat), time.Time{}, `"abc"`, false},
		{"sub-second last modified", "", lastModified.Format(http.TimeFormat), lastModified.Add(987 * time.Millisecond), `"abc"`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.ifNoneMatch != "" {
				r.Header.Set("If-None-Match", tt.ifNoneMatch)
			}
			if tt.ifModifiedSince != "" {
				r.Header.Set("If-Modified-Since", tt.ifModifiedSince)
			}

			assert.Equal(t, tt.expected, notModified(r, tt.etag, tt.lastModified))
		})
	}
}
