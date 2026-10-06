package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePushInterleavedFlags(t *testing.T) {
	for _, args := range [][]string{{"dist", "--name", "x", "--version", "1.0.0"}, {"--name", "x", "--version", "1.0.0", "dist"}} {
		name, packageVersion, _, _, dir, err := parsePush(args)
		assert.NoError(t, err)
		assert.Equal(t, "x", name)
		assert.Equal(t, "1.0.0", packageVersion)
		assert.Equal(t, "dist", dir)
	}
	_, _, _, _, _, err := parsePush([]string{"dist", "--name", "--version", "1.0.0"})
	assert.ErrorContains(t, err, "requires a value")
}

func TestRunUsageAndVersion(t *testing.T) {
	tests := []struct {
		args []string
		env  map[string]string
	}{{[]string{"unknown"}, nil}, {[]string{"push", "dist"}, map[string]string{"GIMME_URL": "x", "GIMME_TOKEN": "x"}}, {[]string{"push", "dist", "--name", "x"}, map[string]string{"GIMME_TOKEN": "x"}}, {[]string{"push", "dist", "--name", "x"}, map[string]string{"GIMME_URL": "x"}}}
	for _, test := range tests {
		var out, stderr bytes.Buffer
		env := func(key string) string { return test.env[key] }
		assert.Equal(t, 2, run(test.args, env, &out, &stderr))
		assert.Contains(t, stderr.String(), "usage:")
	}
	var out, stderr bytes.Buffer
	assert.Equal(t, 0, run([]string{"version"}, func(string) string { return "" }, &out, &stderr))
	assert.Equal(t, version+"\n", out.String())
}

func TestRunUsesEnvironmentFallback(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(dir+"/app.js", []byte("app"), 0o644))
	env := func(key string) string {
		if key == "GIMME_URL" {
			return "://invalid"
		}
		if key == "GIMME_TOKEN" {
			return "secret"
		}
		return ""
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"push", dir, "--name", "pkg", "--version", "1.0.0"}, env, &stdout, &stderr)
	assert.Equal(t, 1, code)
	assert.NotContains(t, stderr.String(), "is required")
}
