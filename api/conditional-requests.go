package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/ziggornif/gimme/internal/content"
)

func cacheControlHeader(version string) string {
	if content.IsPinnedVersion(version) {
		return "public, max-age=31536000, immutable"
	}
	return "public, max-age=300"
}

// etagMatches reports whether an If-None-Match header value matches etag.
// Both sides are compared weakly: the W/ prefix is ignored.
func etagMatches(ifNoneMatch string, etag string) bool {
	if ifNoneMatch == "" || etag == "" {
		return false
	}

	etag = strings.TrimPrefix(etag, "W/")
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}

	return false
}

// notModified reports whether the request may be answered with 304.
func notModified(r *http.Request, etag string, lastModified time.Time) bool {
	if ifNoneMatch := r.Header.Get("If-None-Match"); ifNoneMatch != "" {
		return etagMatches(ifNoneMatch, etag)
	}

	ifModifiedSince := r.Header.Get("If-Modified-Since")
	if ifModifiedSince == "" || lastModified.IsZero() {
		return false
	}

	modifiedSince, err := http.ParseTime(ifModifiedSince)
	if err != nil {
		return false
	}

	return !lastModified.Truncate(time.Second).After(modifiedSince)
}
