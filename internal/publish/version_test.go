package publish

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionResolution(t *testing.T) {
	tests := []struct{ name, explicit, pkg, refType, refName, git, want, source string }{
		{"explicit precedence", "v1.0.0", `{"version":"2.0.0"}`, "tag", "v3.0.0", "v4.0.0", "1.0.0", "--version"},
		{"package", "", `{"version":"v2.0.0"}`, "tag", "v3.0.0", "v4.0.0", "2.0.0", "package.json"},
		{"tag", "", `{invalid`, "tag", "v3.0.0", "v4.0.0", "3.0.0", "GitHub Actions tag"},
		{"git", "", `{}`, "branch", "", "vv4", "v4", "git tag"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if test.pkg != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(test.pkg), 0o644))
			}
			env := func(key string) string {
				if key == "GITHUB_REF_TYPE" {
					return test.refType
				}
				return test.refName
			}
			got, source, err := ResolveVersion(test.explicit, VersionSources{WorkingDir: dir, LookupEnv: env, RunGit: func() (string, error) { return test.git, nil }})
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
			assert.Equal(t, test.source, source)
		})
	}
}

func TestVersionMissingFallsThroughAndNothingFound(t *testing.T) {
	value, _, err := ResolveVersion("", VersionSources{WorkingDir: t.TempDir(), LookupEnv: func(string) string { return "" }, RunGit: func() (string, error) { return "", errors.New("no tag") }})
	assert.Empty(t, value)
	assert.ErrorContains(t, err, "four sources")
}

func TestVersionPackageJSONFallthrough(t *testing.T) {
	for _, contents := range []string{"", "{invalid", `{}`, `{"version":""}`} {
		t.Run(contents, func(t *testing.T) {
			dir := t.TempDir()
			if contents != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(contents), 0o644))
			}
			value, source, err := ResolveVersion("", VersionSources{WorkingDir: dir, LookupEnv: func(key string) string {
				if key == "GITHUB_REF_TYPE" {
					return "tag"
				}
				return "v5.0.0"
			}})
			require.NoError(t, err)
			assert.Equal(t, "5.0.0", value)
			assert.Equal(t, "GitHub Actions tag", source)
		})
	}
}
