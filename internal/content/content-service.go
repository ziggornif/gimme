package content

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/gimme-cdn/gimme/internal/cache"
	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/metrics"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
)

type ContentService struct {
	objectStorageManager storage.ObjectStorageManager
	cacheManager         cache.CacheManager // nil = cache disabled
	cacheTTL             time.Duration
}

type File struct {
	Name   string
	Size   int64
	Folder bool
}

var re = regexp.MustCompile(`^[a-zA-Z0-9-_]+`)

// NewContentService create a new content service instance.
// cacheManager may be nil to disable caching.
func NewContentService(objectStorageManager storage.ObjectStorageManager, cacheManager cache.CacheManager, cacheTTL time.Duration) ContentService {
	return ContentService{
		objectStorageManager: objectStorageManager,
		cacheManager:         cacheManager,
		cacheTTL:             cacheTTL,
	}
}

// filterArray filter objects array
func (svc *ContentService) filterArray(arr []minio.ObjectInfo, fileName string, version string) []minio.ObjectInfo {
	var filtered []minio.ObjectInfo
	for _, item := range arr {
		if strings.Contains(item.Key, fileName) && strings.Contains(item.Key, version) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// getVersion get package version
func (svc *ContentService) getVersion(objStorageFile string) string {
	return strings.Split(strings.Split(objStorageFile, "@")[1], "/")[0]
}

// getLatestVersion get last package version
func (svc *ContentService) getLatestVersion(arr []minio.ObjectInfo) string {
	var versions []string
	for _, curr := range arr {
		versions = append(versions, svc.getVersion(curr.Key))
	}
	if len(versions) == 0 {
		return ""
	}
	semver.Sort(versions)
	return versions[len(versions)-1]
}

// getLatestPackagePath get latest package path
func (svc *ContentService) getLatestPackagePath(ctx context.Context, pkg string, version string, fileName string) string {
	objs := svc.objectStorageManager.ListObjects(ctx, fmt.Sprintf("%s@%s", pkg, version))
	filtred := svc.filterArray(objs, fileName, version)

	if len(filtred) == 0 {
		return fmt.Sprintf("%s@%s%s", pkg, version, fileName)
	}

	lversion := svc.getLatestVersion(filtred)
	return fmt.Sprintf("%s@%s%s", pkg, lversion, fileName)
}

// CreatePackage create package
func (svc *ContentService) CreatePackage(ctx context.Context, name string, version string, file io.ReaderAt, fileSize int64) *errors.GimmeError {
	archive, err := zip.NewReader(file, fileSize)
	if err != nil {
		logrus.Error("[UploadManager] ArchiveProcessor - Error while reading zip file", err)
		return errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while reading zip file"))
	}

	folderName := fmt.Sprintf("%s@%s", name, version)

	if exists := svc.objectStorageManager.ObjectExists(ctx, folderName); exists {
		return errors.NewBusinessError(errors.Conflict, fmt.Errorf("the package %v already exists", folderName))
	}

	var eg errgroup.Group

	for _, currentFile := range archive.File {
		eg.Go(func() error {
			logrus.Debug("[UploadManager] ArchiveProcessor - Unzipping file ", currentFile.Name)
			fileName := re.ReplaceAllString(currentFile.FileHeader.Name, folderName)
			if err := svc.objectStorageManager.AddObject(ctx, fileName, currentFile); err != nil {
				logrus.Errorf("[UploadManager] ArchiveProcessor - Error while processing file %s", fileName)
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
