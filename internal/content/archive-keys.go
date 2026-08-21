package content

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"path"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/sirupsen/logrus"
	"github.com/ziggornif/gimme/internal/errors"
	"golang.org/x/text/unicode/norm"
)

const gimmeIgnoreName = ".gimmeignore"

type archiveObject struct {
	key  string
	file *zip.File
}

type archiveFile struct {
	name     string
	original string
	file     *zip.File
}

// archiveIgnore is the ignore file selected for an archive, with the prefix its
// patterns are matched against — empty when it sits at the archive root.
type archiveIgnore struct {
	file        archiveFile
	matchPrefix string
}

// archiveKeys maps archive entries to object keys under the package namespace.
func archiveKeys(files []*zip.File, folderName string) ([]archiveObject, *errors.GimmeError) {
	validated, err := validatedEntries(files)
	if err != nil {
		return nil, err
	}

	publishable, err := applyIgnoreFile(validated)
	if err != nil {
		return nil, err
	}

	return objectKeys(publishable, folderName)
}

// validatedEntries rejects entry names that escape the package namespace and drops
// the macOS metadata no one wants served. Directory entries produce no object.
func validatedEntries(files []*zip.File) ([]archiveFile, *errors.GimmeError) {
	entries := make([]archiveFile, 0, len(files))
	droppedJunk := 0
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

		cleaned := path.Clean(norm.NFC.String(original))
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive entry %q escapes the package namespace", original))
		}
		if isArchiveJunk(cleaned) {
			droppedJunk++
			continue
		}
		entries = append(entries, archiveFile{name: cleaned, original: original, file: file})
	}
	if droppedJunk > 0 {
		logrus.Infof("[ContentService] validatedEntries - dropped %d junk entries (__MACOSX or .DS_Store)", droppedJunk)
	}

	if len(entries) == 0 {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive contains no files"))
	}
	return entries, nil
}

func isArchiveJunk(cleaned string) bool {
	firstSegment, _, _ := strings.Cut(cleaned, "/")
	return firstSegment == "__MACOSX" || path.Base(cleaned) == ".DS_Store"
}

// singleTopLevelFolder reports the folder every entry is nested under, when there is
// exactly one. An empty set has no such folder.
func singleTopLevelFolder(entries []archiveFile) (string, bool) {
	if len(entries) == 0 {
		return "", false
	}
	common := ""
	for i, entry := range entries {
		root, _, found := strings.Cut(entry.name, "/")
		if !found {
			return "", false
		}
		if i == 0 {
			common = root
		} else if root != common {
			return "", false
		}
	}
	return common, true
}

// findIgnoreFile looks for a .gimmeignore in the two places it is honoured: the
// archive root, then inside the single top-level folder when there is one. A file
// found anywhere else is an ordinary entry.
func findIgnoreFile(entries []archiveFile) (archiveIgnore, bool) {
	for _, entry := range entries {
		if entry.name == gimmeIgnoreName {
			return archiveIgnore{file: entry}, true
		}
	}

	wrapper, hasWrapper := singleTopLevelFolder(entries)
	if !hasWrapper {
		return archiveIgnore{}, false
	}

	nested := wrapper + "/" + gimmeIgnoreName
	for _, entry := range entries {
		if entry.name == nested {
			return archiveIgnore{file: entry, matchPrefix: wrapper + "/"}, true
		}
	}
	return archiveIgnore{}, false
}

// applyIgnoreFile drops the entries excluded by the archive's .gimmeignore, and the
// ignore file itself. Entries are returned untouched when the archive carries none.
func applyIgnoreFile(entries []archiveFile) ([]archiveFile, *errors.GimmeError) {
	selected, found := findIgnoreFile(entries)
	if !found {
		return entries, nil
	}

	patterns, err := readIgnorePatterns(selected.file)
	if err != nil {
		return nil, err
	}

	matcher := ignore.CompileIgnoreLines(patterns...)
	kept := make([]archiveFile, 0, len(entries)-1)
	dropped := 0
	for _, entry := range entries {
		if entry.name == selected.file.name {
			continue
		}
		if matcher.MatchesPath(strings.TrimPrefix(entry.name, selected.matchPrefix)) {
			dropped++
			continue
		}
		kept = append(kept, entry)
	}
	if dropped > 0 {
		logrus.Infof("[ContentService] applyIgnoreFile - dropped %d entries excluded by %s", dropped, selected.file.name)
	}

	if len(kept) == 0 {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive contains no files"))
	}
	return kept, nil
}

func readIgnorePatterns(file archiveFile) ([]string, *errors.GimmeError) {
	reader, err := file.file.Open()
	if err != nil {
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while opening %q: %w", file.original, err))
	}
	defer func(src io.ReadCloser) {
		if err := src.Close(); err != nil {
			logrus.Errorf("[ContentService] readIgnorePatterns - Fail to close %s", file.name)
		}
	}(reader)

	contents, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while reading %q: %w", file.original, err))
	}
	return strings.Split(string(contents), "\n"), nil
}

// objectKeys maps entries into the package namespace, stripping the wrapper folder
// when the archive has one, and rejects entries colliding on the same key.
func objectKeys(entries []archiveFile, folderName string) ([]archiveObject, *errors.GimmeError) {
	wrapper, stripWrapper := singleTopLevelFolder(entries)

	prefix := folderName + "/"
	objects := make([]archiveObject, 0, len(entries))
	originalByKey := make(map[string]string, len(entries))
	for _, entry := range entries {
		relativePath := entry.name
		if stripWrapper {
			relativePath = strings.TrimPrefix(relativePath, wrapper+"/")
		}
		key := prefix + relativePath
		if !strings.HasPrefix(key, prefix) {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive entry %q escapes the package namespace", entry.original))
		}
		if previous, exists := originalByKey[key]; exists {
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("archive entries %q and %q map to duplicate key %q", previous, entry.original, key))
		}
		originalByKey[key] = entry.original
		objects = append(objects, archiveObject{key: key, file: entry.file})
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
