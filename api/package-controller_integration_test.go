package api

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ziggornif/gimme/internal/auth"
	"github.com/ziggornif/gimme/internal/content"
	"github.com/ziggornif/gimme/test/mocks"
	"github.com/ziggornif/gimme/test/utils"
)

// archiveWithEntries builds a ZIP carrying arbitrary entries, so a test can supply
// its own precompressed variant alongside the identity file.
func archiveWithEntries(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.zip")
	file, err := os.Create(path)
	require.NoError(t, err)
	w := zip.NewWriter(file)
	for name, data := range entries {
		entry, err := w.Create(name)
		require.NoError(t, err)
		_, err = entry.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, file.Close())
	return path
}

func compressionArchive(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.zip")
	file, err := os.Create(path)
	require.NoError(t, err)
	w := zip.NewWriter(file)
	entry, err := w.Create(name)
	require.NoError(t, err)
	_, err = entry.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, file.Close())
	return path
}

func decodeResponse(t *testing.T, encoding string, body io.Reader) []byte {
	t.Helper()
	if encoding == "br" {
		decoded, err := io.ReadAll(brotli.NewReader(body))
		require.NoError(t, err)
		return decoded
	}
	reader, err := gzip.NewReader(body)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	decoded, err := io.ReadAll(reader)
	require.NoError(t, err)
	return decoded
}

func TestPackageControllerCompression(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{}, content.WithCompression(true))
	NewPackageController(router, authManager, service)

	source := []byte(strings.Repeat("const answer = 42;\n", 256))
	archive := compressionArchive(t, "app.js", source)
	resp := createPackage(t, router, "compressed", "1.0.0", archive, rawToken)
	require.Equal(t, http.StatusCreated, resp.Code)
	t.Cleanup(func() { _ = service.DeletePackage(context.Background(), "compressed", "1.0.0") })

	path := "/gimme/compressed@1.0.0/app.js"
	identity := utils.PerformRequest(router, "GET", path, nil)
	assert.Equal(t, http.StatusOK, identity.Code)
	assert.Empty(t, identity.Header().Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", identity.Header().Get("Vary"))
	assert.Equal(t, source, identity.Body.Bytes())

	for _, encoding := range []string{"br", "gzip"} {
		t.Run(encoding, func(t *testing.T) {
			response := utils.PerformRequest(router, "GET", path, nil, utils.Header{Key: "Accept-Encoding", Value: encoding})
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, encoding, response.Header().Get("Content-Encoding"))
			assert.Equal(t, "Accept-Encoding", response.Header().Get("Vary"))
			assert.Equal(t, source, decodeResponse(t, encoding, response.Body))
		})
	}

	brotliResponse := utils.PerformRequest(router, "GET", path, nil, utils.Header{Key: "Accept-Encoding", Value: "br"})
	assert.NotEqual(t, identity.Header().Get("ETag"), brotliResponse.Header().Get("ETag"))
	notModified := utils.PerformRequest(router, "GET", path, nil,
		utils.Header{Key: "Accept-Encoding", Value: "br"},
		utils.Header{Key: "If-None-Match", Value: brotliResponse.Header().Get("ETag")})
	assert.Equal(t, http.StatusNotModified, notModified.Code)
	assert.Equal(t, "Accept-Encoding", notModified.Header().Get("Vary"))

	listing := utils.PerformRequest(router, "GET", "/gimme/compressed@1.0.0/", nil)
	assert.NotContains(t, listing.Body.String(), "app.js.br")
	assert.NotContains(t, listing.Body.String(), "app.js.gz")
}

func TestPackageControllerIntegrity(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{}, content.WithCompression(true))
	NewPackageController(router, authManager, service)

	source := []byte(strings.Repeat("const integrity = true;\n", 256))
	resp := createPackage(t, router, "integrity", "1.0.0", compressionArchive(t, "app.js", source), rawToken)
	require.Equal(t, http.StatusCreated, resp.Code)
	t.Cleanup(func() { _ = service.DeletePackage(context.Background(), "integrity", "1.0.0") })

	digest := sha512.Sum384(source)
	expected := "sha384-" + base64.StdEncoding.EncodeToString(digest[:])
	path := "/gimme/integrity@1.0.0/app.js"

	identity := utils.PerformRequest(router, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, identity.Code)
	assert.Equal(t, source, identity.Body.Bytes())
	assert.Equal(t, expected, identity.Header().Get("Gimme-Integrity"))

	brotliResponse := utils.PerformRequest(router, http.MethodGet, path, nil, utils.Header{Key: "Accept-Encoding", Value: "br"})
	require.Equal(t, http.StatusOK, brotliResponse.Code)
	assert.Equal(t, "br", brotliResponse.Header().Get("Content-Encoding"))
	assert.Equal(t, expected, brotliResponse.Header().Get("Gimme-Integrity"))

	head := utils.PerformRequest(router, http.MethodHead, path, nil)
	require.Equal(t, http.StatusOK, head.Code)
	assert.Equal(t, expected, head.Header().Get("Gimme-Integrity"))
	assert.Zero(t, head.Body.Len())

	notModified := utils.PerformRequest(router, http.MethodGet, path, nil, utils.Header{Key: "If-None-Match", Value: identity.Header().Get("ETag")})
	require.Equal(t, http.StatusNotModified, notModified.Code)
	assert.Equal(t, expected, notModified.Header().Get("Gimme-Integrity"))

	folderGet := utils.PerformRequest(router, http.MethodGet, "/gimme/integrity@1.0.0/", nil)
	require.Equal(t, http.StatusOK, folderGet.Code)
	require.NotZero(t, folderGet.Body.Len())

	folder := utils.PerformRequest(router, http.MethodHead, "/gimme/integrity@1.0.0/", nil)
	require.Equal(t, http.StatusOK, folder.Code)
	assert.Zero(t, folder.Body.Len())
	assert.Equal(t, folderGet.Header().Get("Content-Type"), folder.Header().Get("Content-Type"))

	missingFolder := utils.PerformRequest(router, http.MethodHead, "/gimme/integrity@9.9.9/", nil)
	assert.Equal(t, http.StatusNotFound, missingFolder.Code)
}

func TestPackageControllerIntegrityArchiveSuppliedVariant(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{}, content.WithCompression(true))
	NewPackageController(router, authManager, service)

	source := []byte(strings.Repeat("const supplied = true;\n", 256))
	var gzipped bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipped)
	_, writeErr := gzipWriter.Write(source)
	require.NoError(t, writeErr)
	require.NoError(t, gzipWriter.Close())

	archive := archiveWithEntries(t, map[string][]byte{
		"app.js":    source,
		"app.js.gz": gzipped.Bytes(),
	})
	resp := createPackage(t, router, "supplied", "1.0.0", archive, rawToken)
	require.Equal(t, http.StatusCreated, resp.Code)
	t.Cleanup(func() { _ = service.DeletePackage(context.Background(), "supplied", "1.0.0") })

	digest := sha512.Sum384(source)
	expected := "sha384-" + base64.StdEncoding.EncodeToString(digest[:])

	path := "/gimme/supplied@1.0.0/app.js"
	identity := utils.PerformRequest(router, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, identity.Code)
	require.Equal(t, expected, identity.Header().Get("Gimme-Integrity"))

	// The archive supplied this variant, so its stored bytes are not the ones SRI
	// validates: the digest must still describe the decoded body.
	encoded := utils.PerformRequest(router, http.MethodGet, path, nil, utils.Header{Key: "Accept-Encoding", Value: "gzip"})
	require.Equal(t, http.StatusOK, encoded.Code)
	require.Equal(t, "gzip", encoded.Header().Get("Content-Encoding"))
	assert.Equal(t, expected, encoded.Header().Get("Gimme-Integrity"))
}

func TestPackageControllerLegacyObjectWithoutIntegrity(t *testing.T) {
	objectStorageManager := initObjectStorage()
	require.Nil(t, objectStorageManager.AddBytes(context.Background(), "legacy@1.0.0/app.js", []byte("legacy"), "application/javascript", ""))
	t.Cleanup(func() { _ = objectStorageManager.RemoveObjects(context.Background(), "legacy@1.0.0/") })

	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	response := utils.PerformRequest(router, http.MethodGet, "/gimme/legacy@1.0.0/app.js", nil)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("Gimme-Integrity"))
	assert.Equal(t, "legacy", response.Body.String())
}

func TestPackageControllerCompressionFallback(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	source := []byte(strings.Repeat("const answer = 42;\n", 256))
	resp := createPackage(t, router, "identity", "1.0.0", compressionArchive(t, "app.js", source), rawToken)
	require.Equal(t, http.StatusCreated, resp.Code)
	t.Cleanup(func() { _ = service.DeletePackage(context.Background(), "identity", "1.0.0") })

	response := utils.PerformRequest(router, "GET", "/gimme/identity@1.0.0/app.js", nil, utils.Header{Key: "Accept-Encoding", Value: "br"})
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("Content-Encoding"))
	assert.Equal(t, source, response.Body.Bytes())
}

func TestPackageControllerCompressionSmallFileFallback(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{}, content.WithCompression(true))
	NewPackageController(router, authManager, service)

	source := []byte("const answer = 42;")
	resp := createPackage(t, router, "small", "1.0.0", compressionArchive(t, "app.js", source), rawToken)
	require.Equal(t, http.StatusCreated, resp.Code)
	t.Cleanup(func() { _ = service.DeletePackage(context.Background(), "small", "1.0.0") })

	response := utils.PerformRequest(router, "GET", "/gimme/small@1.0.0/app.js", nil, utils.Header{Key: "Accept-Encoding", Value: "br"})
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("Content-Encoding"))
	assert.Equal(t, source, response.Body.Bytes())
}

func TestPackageControllerCompressionIdentityRefused(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{}, content.WithCompression(true))
	NewPackageController(router, authManager, service)

	source := []byte(strings.Repeat("const answer = 42;\n", 256))
	resp := createPackage(t, router, "refused", "1.0.0", compressionArchive(t, "app.js", source), rawToken)
	require.Equal(t, http.StatusCreated, resp.Code)
	t.Cleanup(func() { _ = service.DeletePackage(context.Background(), "refused", "1.0.0") })

	path := "/gimme/refused@1.0.0/app.js"

	// A refused identity with an acceptable variant is served as that variant.
	served := utils.PerformRequest(router, "GET", path, nil, utils.Header{Key: "Accept-Encoding", Value: "identity;q=0, br"})
	assert.Equal(t, http.StatusOK, served.Code)
	assert.Equal(t, "br", served.Header().Get("Content-Encoding"))

	// A refused identity with no acceptable coding left has no representation to send.
	for _, header := range []string{"identity;q=0", "*;q=0"} {
		refused := utils.PerformRequest(router, "GET", path, nil, utils.Header{Key: "Accept-Encoding", Value: header})
		assert.Equal(t, http.StatusNotAcceptable, refused.Code, header)
		assert.Equal(t, "no-store", refused.Header().Get("Cache-Control"), header)
	}
}

func newTestAuthManager(t *testing.T) *auth.AuthManager {
	t.Helper()
	store, err := auth.NewFileTokenStore("this-is-a-32-byte-secret-for-test", filepath.Join(t.TempDir(), "tokens.enc"))
	if err != nil {
		t.Fatalf("newTestAuthManager: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return auth.NewAuthManager(store)
}

func TestPackageControllerGETInvalidUrlErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/file.js", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerGETInvalidUrlAlterErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/foo/bar.js", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerRedirect(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme", nil)

	assert.Equal(t, http.StatusFound, w.Code)
}

func TestPackageControllerCreate(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	resp := createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusCreated, resp.Code)

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGet(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0/awesome-lib.min.js", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "javascript")

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetConditional(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	for _, version := range []string{"1.0.0", "1.0"} {
		t.Run(version, func(t *testing.T) {
			path := "/gimme/awesome-lib@" + version + "/awesome-lib.min.js"
			first := utils.PerformRequest(router, "GET", path, nil)

			assert.Equal(t, http.StatusOK, first.Code)
			etag := first.Header().Get("ETag")
			assert.NotEmpty(t, etag)
			assert.NotEmpty(t, first.Header().Get("Last-Modified"))

			notModified := utils.PerformRequest(router, "GET", path, nil,
				utils.Header{Key: "If-None-Match", Value: etag})
			assert.Equal(t, http.StatusNotModified, notModified.Code)
			assert.Zero(t, notModified.Body.Len())
			assert.NotEmpty(t, notModified.Header().Get("Cache-Control"))
			assert.Equal(t, etag, notModified.Header().Get("ETag"))

			stale := utils.PerformRequest(router, "GET", path, nil,
				utils.Header{Key: "If-None-Match", Value: `"nope"`})
			assert.Equal(t, http.StatusOK, stale.Code)
			assert.NotZero(t, stale.Body.Len())
		})
	}

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetUI(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetUIAlter(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0/", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerCreateConflictErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	resp := createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusCreated, resp.Code)

	resp2 := createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusConflict, resp2.Code)

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetEmpty(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@4.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerGetNotFound(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/invalid@1.0.0/invalid.js", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerPOSTEmptyFile(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	err := writer.WriteField("name", "awesome-lib")
	assert.Nil(t, err)
	err = writer.WriteField("version", "1.0.0")
	assert.Nil(t, err)
	err = writer.Close()
	assert.Nil(t, err)

	w := utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)},
		utils.Header{
			Key: "Content-Type", Value: writer.FormDataContentType(),
		})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerDeleteInvalidUrlErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "DELETE", "/packages/file.js", nil,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerDelete(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "DELETE", "/packages/awesome-lib@1.0.0", nil,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPackageControllerCreateFailureRollsBack(t *testing.T) {
	objectStorageManager := initObjectStorage()
	failing := &mocks.MockOSManagerFailAfter{OSManager: objectStorageManager, FailAt: 2}

	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(failing, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	t.Cleanup(func() {
		_ = objectStorageManager.RemoveObjects(context.Background(), "rollback-lib@1.0.0/") //nolint:errcheck
	})

	resp := createPackage(t, router, "rollback-lib", "1.0.0", "../test/test.zip", rawToken)
	require.Equal(t, http.StatusInternalServerError, resp.Code)

	remaining := objectStorageManager.ListObjects(context.Background(), "rollback-lib@1.0.0/")
	assert.Empty(t, remaining, "a failed upload must leave no object behind")

	healthyRouter := gin.New()
	healthyService := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(healthyRouter, authManager, healthyService)

	retry := createPackage(t, healthyRouter, "rollback-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusCreated, retry.Code)
}
