package publish

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VersionSources supplies the inputs used to resolve a package version.
type VersionSources struct {
	WorkingDir string
	LookupEnv  func(string) string
	RunGit     func() (string, error)
}

// ResolveVersion selects the first available package version and its source.
func ResolveVersion(explicit string, sources VersionSources) (string, string, error) {
	if explicit != "" {
		return stripV(explicit), "--version", nil
	}
	if data, err := os.ReadFile(filepath.Join(sources.WorkingDir, "package.json")); err == nil {
		var pkg struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Version != "" {
			return stripV(pkg.Version), "package.json", nil
		}
	}
	if sources.LookupEnv != nil && sources.LookupEnv("GITHUB_REF_TYPE") == "tag" {
		if value := sources.LookupEnv("GITHUB_REF_NAME"); value != "" {
			return stripV(value), "GitHub Actions tag", nil
		}
	}
	if sources.RunGit != nil {
		if value, err := sources.RunGit(); err == nil && strings.TrimSpace(value) != "" {
			return stripV(strings.TrimSpace(value)), "git tag", nil
		}
	}
	return "", "", fmt.Errorf("could not determine version from the four sources: --version, package.json, GitHub Actions tag, or git tag")
}

func stripV(value string) string {
	return strings.TrimPrefix(value, "v")
}
