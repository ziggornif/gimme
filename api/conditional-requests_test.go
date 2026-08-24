package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
