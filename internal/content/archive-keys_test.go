package content

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// entriesNamed builds the only part of archiveFile these phases read.
func entriesNamed(names ...string) []archiveFile {
	entries := make([]archiveFile, 0, len(names))
	for _, name := range names {
		entries = append(entries, archiveFile{name: name, original: name})
	}
	return entries
}

func TestSingleTopLevelFolder(t *testing.T) {
	tests := []struct {
		name    string
		entries []archiveFile
		folder  string
		found   bool
	}{
		{
			// Not reachable through CreatePackage: every caller rejects an empty
			// archive first. Pinned here so the guarantee stays in the function.
			name:    "no entries have no common folder",
			entries: nil,
			folder:  "",
			found:   false,
		},
		{
			name:    "every entry under one folder",
			entries: entriesNamed("dist/app.js", "dist/css/style.css"),
			folder:  "dist",
			found:   true,
		},
		{
			name:    "a root level file means there is no wrapper",
			entries: entriesNamed("dist/app.js", "README.md"),
			folder:  "",
			found:   false,
		},
		{
			name:    "two top level folders",
			entries: entriesNamed("src/a.js", "docs/b.md"),
			folder:  "",
			found:   false,
		},
		{
			name:    "a single root level file",
			entries: entriesNamed("app.js"),
			folder:  "",
			found:   false,
		},
		{
			name:    "a folder whose name prefixes another",
			entries: entriesNamed("dist/app.js", "distro/b.js"),
			folder:  "",
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folder, found := singleTopLevelFolder(tt.entries)
			assert.Equal(t, tt.found, found)
			assert.Equal(t, tt.folder, folder)
		})
	}
}

func TestFindIgnoreFile(t *testing.T) {
	tests := []struct {
		name        string
		entries     []archiveFile
		found       bool
		file        string
		matchPrefix string
	}{
		{
			name:    "no ignore file",
			entries: entriesNamed("app.js", "img/logo.svg"),
			found:   false,
		},
		{
			name:        "at the archive root",
			entries:     entriesNamed(".gimmeignore", "app.js"),
			found:       true,
			file:        ".gimmeignore",
			matchPrefix: "",
		},
		{
			name:        "inside the single wrapper folder",
			entries:     entriesNamed("dist/.gimmeignore", "dist/app.js"),
			found:       true,
			file:        "dist/.gimmeignore",
			matchPrefix: "dist/",
		},
		{
			name:        "the archive root wins over the wrapper folder",
			entries:     entriesNamed(".gimmeignore", "dist/.gimmeignore", "dist/app.js"),
			found:       true,
			file:        ".gimmeignore",
			matchPrefix: "",
		},
		{
			name:    "deeper than the wrapper folder is not honoured",
			entries: entriesNamed("dist/sub/.gimmeignore", "dist/app.js"),
			found:   false,
		},
		{
			// No wrapper, so the folder holding it is not a lookup location.
			name:    "in a folder that is not the wrapper is not honoured",
			entries: entriesNamed("app.js", "sub/.gimmeignore"),
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, found := findIgnoreFile(tt.entries)
			assert.Equal(t, tt.found, found)
			if !tt.found {
				return
			}
			assert.Equal(t, tt.file, selected.file.name)
			assert.Equal(t, tt.matchPrefix, selected.matchPrefix)
		})
	}
}

func TestIsArchiveJunk(t *testing.T) {
	junk := []string{
		"__MACOSX/dist/._app.js",
		"__MACOSX",
		".DS_Store",
		"dist/.DS_Store",
		"dist/css/.DS_Store",
	}
	for _, name := range junk {
		assert.True(t, isArchiveJunk(name), "%q must be treated as junk", name)
	}

	kept := []string{
		"app.js",
		"dist/._app.js",
		"._app.js",
		".well-known/probe.txt",
		"dist/DS_Store",
		"my__MACOSX/app.js",
		"dist/.DS_Store.js",
	}
	for _, name := range kept {
		assert.False(t, isArchiveJunk(name), "%q must be kept", name)
	}
}
