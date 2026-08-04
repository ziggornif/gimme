package content

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"math"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"github.com/ziggornif/gimme/internal/cache"
	"github.com/ziggornif/gimme/internal/errors"
	"github.com/ziggornif/gimme/internal/metrics"
	"github.com/ziggornif/gimme/internal/storage"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
)

type ContentService struct {
	objectStorageManager storage.ObjectStorageManager
	cacheManager         cache.CacheManager // nil = cache disabled
	cacheTTL             time.Duration
	uploadLimits         UploadLimits
}

// UploadLimits controls upload checks; a non-positive field disables that check.
type UploadLimits struct {
	MaxSize             int64
	MaxEntries          int
	MaxUncompressedSize int64
}

type File struct {
	Name   string
	Size   int64
	Folder bool
}

// NewContentService create a new content service instance.
// cacheManager may be nil to disable caching.
func NewContentService(objectStorageManager storage.ObjectStorageManager, cacheManager cache.CacheManager, cacheTTL time.Duration, limits UploadLimits) ContentService {
	return ContentService{
		objectStorageManager: objectStorageManager,
		cacheManager:         cacheManager,
		cacheTTL:             cacheTTL,
		uploadLimits:         limits,
	}
}

// UploadLimits returns the limits the service enforces, so the HTTP layer can
// cap the request body with the same value.
func (svc *ContentService) UploadLimits() UploadLimits {
	return svc.uploadLimits
}

// filterArray filter objects array
func (svc *ContentService) filterArray(arr []minio.ObjectInfo, pkg string, fileName string, version string) []minio.ObjectInfo {
	var filtered []minio.ObjectInfo
	packagePrefix := fmt.Sprintf("%s@", pkg)
	partialVersion := len(strings.Split(version, ".")) < 3

	for _, item := range arr {
		keyWithoutPackage, found := strings.CutPrefix(item.Key, packagePrefix)
		if !found {
			continue
		}

		candidateVersion, candidateFile, found := strings.Cut(keyWithoutPackage, "/")
		if !found || "/"+candidateFile != fileName {
			continue
		}

		candidateSemver := "v" + candidateVersion
		if !semver.IsValid(candidateSemver) {
			continue
		}

		matchesVersion := candidateVersion == version
		if partialVersion && semver.Prerelease(candidateSemver) == "" {
			switch len(strings.Split(version, ".")) {
			case 1:
				matchesVersion = semver.Major(candidateSemver) == "v"+version
			case 2:
				matchesVersion = semver.MajorMinor(candidateSemver) == "v"+version
			}
		}
		if matchesVersion {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// getVersion get package version from an S3 object key.
// Returns an empty string if the key does not contain the expected '@' separator
// (defensive: avoids a panic on malformed/unexpected object names).
func (svc *ContentService) getVersion(objStorageFile string) string {
	parts := strings.SplitN(objStorageFile, "@", 2)
	if len(parts) < 2 {
		logrus.Warnf("[ContentService] getVersion - unexpected object key without '@': %s", objStorageFile)
		return ""
	}
	return strings.Split(parts[1], "/")[0]
}

// getLatestVersion get last package version
func (svc *ContentService) getLatestVersion(arr []minio.ObjectInfo) string {
	var versions []string
	for _, curr := range arr {
		v := svc.getVersion(curr.Key)
		if v == "" {
			continue // skip malformed entries
		}
		versions = append(versions, "v"+v)
	}
	if len(versions) == 0 {
		return ""
	}
	semver.Sort(versions)
	return strings.TrimPrefix(versions[len(versions)-1], "v")
}

// getLatestPackagePath get latest package path
func (svc *ContentService) getLatestPackagePath(ctx context.Context, pkg string, version string, fileName string) string {
	fileName = "/" + strings.TrimLeft(fileName, "/")
	objs := svc.objectStorageManager.ListObjects(ctx, fmt.Sprintf("%s@%s", pkg, version))
	filtred := svc.filterArray(objs, pkg, fileName, version)

	if len(filtred) == 0 {
		return fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	}

	lversion := svc.getLatestVersion(filtred)
	return fmt.Sprintf("%s@%s%s", pkg, lversion, fileName)
}

type archiveObject struct {
	key  string
	file *zip.File
}

type archiveFile struct {
	name     string
	original string
	file     *zip.File
}

// archiveKeys validates archive entry names and maps files into the package namespace.
func archiveKeys(files []*zip.File, folderName string) ([]archiveObject, *errors.GimmeError) {
	normalized := make([]archiveFile, 0, len(files))
	for _, file := range files {
		original := file.Name
		if file.FileInfo().IsDir() || strings.HasSuffix(original, "/") {
			continue
		}
		if original == "" {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive contains an empty entry name"))
		}
		if strings.HasPrefix(original, "/") {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive entry %q is an absolute path", original))
		}

		cleaned := path.Clean(original)
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive entry %q escapes the package namespace", original))
		}
		normalized = append(normalized, archiveFile{name: cleaned, original: original, file: file})
	}

	if len(normalized) == 0 {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive contains no files"))
	}

	commonRoot := ""
	stripRoot := true
	for i, file := range normalized {
		root, _, found := strings.Cut(file.name, "/")
		if !found {
			stripRoot = false
			break
		}
		if i == 0 {
			commonRoot = root
		} else if root != commonRoot {
			stripRoot = false
			break
		}
	}

	prefix := folderName + "/"
	objects := make([]archiveObject, 0, len(normalized))
	originalByKey := make(map[string]string, len(normalized))
	for _, file := range normalized {
		relativePath := file.name
		if stripRoot {
			_, relativePath, _ = strings.Cut(relativePath, "/")
		}
		key := prefix + relativePath
		if !strings.HasPrefix(key, prefix) {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive entry %q escapes the package namespace", file.original))
		}
		if previous, exists := originalByKey[key]; exists {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive entries %q and %q map to duplicate key %q", previous, file.original, key))
		}
		originalByKey[key] = file.original
		objects = append(objects, archiveObject{key: key, file: file.file})
	}

	return objects, nil
}

// checkArchiveLimits counts the file entries of an archive and sums their declared
// uncompressed size, then rejects the archive when either limit is exceeded.
// Directory entries are skipped: they produce no object.
func (svc *ContentService) checkArchiveLimits(archive *zip.Reader) *errors.GimmeError {
	var entryCount int
	var uncompressedSize uint64
	for _, archiveFile := range archive.File {
		if archiveFile.FileInfo().IsDir() || strings.HasSuffix(archiveFile.Name, "/") {
			continue
		}
		entryCount++
		if math.MaxUint64-uncompressedSize < archiveFile.UncompressedSize64 {
			uncompressedSize = math.MaxUint64
		} else {
			uncompressedSize += archiveFile.UncompressedSize64
		}
	}

	if svc.uploadLimits.MaxEntries > 0 && entryCount > svc.uploadLimits.MaxEntries {
		return errors.NewBusinessError(errors.PayloadTooLarge, fmt.Errorf("archive holds %d entries, over the limit of %d (upload.max_entries)", entryCount, svc.uploadLimits.MaxEntries))
	}
	if svc.uploadLimits.MaxUncompressedSize > 0 && uncompressedSize > uint64(svc.uploadLimits.MaxUncompressedSize) {
		return errors.NewBusinessError(errors.PayloadTooLarge, fmt.Errorf("archive expands to %d bytes, over the limit of %d (upload.max_uncompressed_size)", uncompressedSize, svc.uploadLimits.MaxUncompressedSize))
	}
	return nil
}

// CreatePackage create package
func (svc *ContentService) CreatePackage(ctx context.Context, name string, version string, file io.ReaderAt, fileSize int64) *errors.GimmeError {
	if svc.uploadLimits.MaxSize > 0 && fileSize > svc.uploadLimits.MaxSize {
		return errors.NewBusinessError(errors.PayloadTooLarge, fmt.Errorf("upload exceeds the maximum request size of %d bytes (upload.max_size)", svc.uploadLimits.MaxSize))
	}

	archive, err := zip.NewReader(file, fileSize)
	if err != nil {
		logrus.Error("[ContentService] CreatePackage - Error while reading zip file", err)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while reading zip file"))
	}

	if limitErr := svc.checkArchiveLimits(archive); limitErr != nil {
		return limitErr
	}

	folderName := fmt.Sprintf("%s@%s", name, version)

	if exists := svc.objectStorageManager.ObjectExists(ctx, folderName); exists {
		return errors.NewBusinessError(errors.Conflict, fmt.Errorf("the package %v already exists", folderName))
	}

	objects, validationErr := archiveKeys(archive.File, folderName)
	if validationErr != nil {
		logrus.Errorf("[ContentService] CreatePackage - Invalid archive: %v", validationErr)
		return validationErr
	}

	var eg errgroup.Group
	// Bound concurrency so resource use does not grow unbounded with archive entry count.
	eg.SetLimit(runtime.NumCPU() * 4)

	for _, object := range objects {
		objectKey := object.key
		currentFile := object.file
		eg.Go(func() error {
			logrus.Debug("[ContentService] CreatePackage - Unzipping file ", currentFile.Name)
			if err := svc.objectStorageManager.AddObject(ctx, objectKey, currentFile); err != nil {
				logrus.Errorf("[ContentService] CreatePackage - Error while processing file %s", objectKey)
				return err.Err
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while uploading package files: %w", err))
	}

	metrics.PackagesUploadedTotal.Inc()
	return nil
}

// IsPinnedVersion returns true if version is an explicit full 3-part semver
// with no pre-release suffix (e.g. "1.0.0", "1.2.3"), meaning the content is
// immutable and can be cached indefinitely.
// Partial versions ("1.0", "1") and pre-release versions ("1.0.0-rc.1") return false.
func IsPinnedVersion(version string) bool {
	// Require exactly 3 dot-separated numeric parts in the original string
	// before any pre-release/build suffix.
	base := strings.SplitN(version, "-", 2)[0]
	base = strings.SplitN(base, "+", 2)[0]
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return false
	}
	// Validate the full version as semver and ensure no pre-release.
	v := "v" + version
	if !semver.IsValid(v) {
		return false
	}
	return semver.Prerelease(v) == ""
}

// GetFile get package file
func (svc *ContentService) GetFile(ctx context.Context, pkg string, version string, fileName string) (*minio.Object, *errors.GimmeError) {
	valid := semver.IsValid(fmt.Sprintf("v%v", version))
	if !valid {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("invalid version (asked version must be semver compatible)"))
	}

	pinned := IsPinnedVersion(version)
	cacheKey := fmt.Sprintf("%s@%s%s", pkg, version, fileName)

	// Cache lookup — only for partial versions (pinned paths are already deterministic)
	if !pinned && svc.cacheManager != nil {
		if entry, ok := svc.cacheManager.Get(ctx, cacheKey); ok {
			logrus.Debugf("[ContentService] GetFile - Cache hit for %s", cacheKey)
			metrics.CacheHitsTotal.Inc()
			return svc.objectStorageManager.GetObject(ctx, entry.ObjectPath)
		}
		metrics.CacheMissesTotal.Inc()
	}

	var objectPath string
	if pinned {
		objectPath = cacheKey
	} else {
		objectPath = svc.getLatestPackagePath(ctx, pkg, version, fileName)
	}

	obj, err := svc.objectStorageManager.GetObject(ctx, objectPath)
	if err != nil {
		return nil, err
	}

	// Store resolved path in cache for future partial-version requests
	if !pinned && svc.cacheManager != nil {
		entry := &cache.CacheEntry{ObjectPath: objectPath}
		if setErr := svc.cacheManager.Set(ctx, cacheKey, entry, svc.cacheTTL); setErr != nil {
			logrus.Warnf("[ContentService] GetFile - Could not store cache entry for %s: %v", cacheKey, setErr)
		} else {
			logrus.Debugf("[ContentService] GetFile - Cached %s → %s", cacheKey, objectPath)
		}
	}

	return obj, nil
}

// GetFiles get package files
func (svc *ContentService) GetFiles(ctx context.Context, pkg string, version string) ([]File, *errors.GimmeError) {
	objs := svc.objectStorageManager.ListObjects(ctx, fmt.Sprintf("%s@%s", pkg, version))

	var files []File
	for _, obj := range objs {
		files = append(files, File{
			Name:   obj.Key,
			Size:   obj.Size,
			Folder: false,
		})
	}
	return files, nil
}

// DeletePackage delete package
func (svc *ContentService) DeletePackage(ctx context.Context, pkg string, version string) *errors.GimmeError {
	prefix := fmt.Sprintf("%s@%s", pkg, version)

	err := svc.objectStorageManager.RemoveObjects(ctx, prefix)
	if err != nil {
		return err
	}

	if svc.cacheManager != nil {
		// Invalidate the exact version prefix (e.g. "pkg@1.0.3").
		if cacheErr := svc.cacheManager.DeleteByPrefix(ctx, prefix); cacheErr != nil {
			logrus.Warnf("[ContentService] DeletePackage - Could not invalidate cache for prefix %s: %v", prefix, cacheErr)
		} else {
			logrus.Debugf("[ContentService] DeletePackage - Invalidated cache entries for prefix %s", prefix)
		}

		// Also invalidate partial version entries that may have resolved to this
		// version (e.g. "pkg@1.0" or "pkg@1" caching a path that pointed to "pkg@1.0.3").
		for _, partialPrefix := range partialVersionPrefixes(pkg, version) {
			if cacheErr := svc.cacheManager.DeleteByPrefix(ctx, partialPrefix); cacheErr != nil {
				logrus.Warnf("[ContentService] DeletePackage - Could not invalidate partial cache for prefix %s: %v", partialPrefix, cacheErr)
			} else {
				logrus.Debugf("[ContentService] DeletePackage - Invalidated partial cache entries for prefix %s", partialPrefix)
			}
		}
	}

	metrics.PackagesDeletedTotal.Inc()
	return nil
}

// partialVersionPrefixes returns the cache key prefixes for partial versions of a package.
// For example, deleting "pkg@1.0.3" should also invalidate "pkg@1.0" and "pkg@1"
// since those partial-version cache entries may have resolved to the deleted version.
// Pre-release and build-metadata suffixes are stripped before computing the partial
// prefixes so that "1.0.0-rc.1" generates the same partial prefixes as "1.0.0".
func partialVersionPrefixes(pkg, version string) []string {
	// Strip pre-release suffix (e.g. "1.0.0-rc.1" → "1.0.0")
	base := strings.SplitN(version, "-", 2)[0]
	// Strip build metadata (e.g. "1.0.0+build.1" → "1.0.0")
	base = strings.SplitN(base, "+", 2)[0]

	parts := strings.Split(base, ".")
	var prefixes []string
	// Build prefixes for each level shorter than the full version: major, major.minor, etc.
	for i := 1; i < len(parts); i++ {
		partial := strings.Join(parts[:i], ".")
		prefixes = append(prefixes, fmt.Sprintf("%s@%s", pkg, partial))
	}
	return prefixes
}
