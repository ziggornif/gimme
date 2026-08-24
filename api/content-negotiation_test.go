package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ziggornif/gimme/internal/content"
)

func TestParseAcceptEncoding(t *testing.T) {
	br := content.EncodingBrotli
	gzip := content.EncodingGzip
	tests := []struct {
		header    string
		encodings []content.Encoding
		identity  bool
	}{
		{"", nil, true},
		{"br", []content.Encoding{br}, true},
		{"gzip", []content.Encoding{gzip}, true},
		{"br, gzip", []content.Encoding{br, gzip}, true},
		{"br;q=0.5, gzip;q=0.9", []content.Encoding{gzip, br}, true},
		{"gzip;q=0", nil, true},
		{"*", []content.Encoding{br, gzip}, true},
		{"deflate, zstd", nil, true},
		{"br;q=nope, gzip", []content.Encoding{gzip}, true},
		// A named entry wins over "*" whatever the order, so an explicit refusal
		// is not resurrected by the wildcard (RFC 9110 section 12.5.3, rule 3).
		{"gzip;q=0, *;q=1", []content.Encoding{br}, true},
		{"*;q=1, gzip;q=0", []content.Encoding{br}, true},
		{"br;q=0, *", []content.Encoding{gzip}, true},
		{"*;q=0.5, br;q=0.1", []content.Encoding{gzip, br}, true},
		{"br;q=0.1, *;q=0.5", []content.Encoding{gzip, br}, true},
		// The unencoded representation is refused only by an entry that names it,
		// or by a wildcard no identity entry overrides.
		// A qvalue that is not a number is a malformed entry, not a refusal:
		// every comparison with NaN is false, so it used to slip past the range
		// check and answer 406 on a servable file.
		{"identity;q=NaN", nil, true},
		{"br;q=NaN", nil, true},
		{"br;q=+Inf", nil, true},
		{"identity;q=0", nil, false},
		{"*;q=0", nil, false},
		{"*;q=0, identity", nil, true},
		{"identity;q=0, br", []content.Encoding{br}, false},
		{"gzip;q=1.0, identity;q=0.5, *;q=0", []content.Encoding{gzip}, true},
	}
	for _, tt := range tests {
		accepted := parseAcceptEncoding(tt.header)
		assert.Equal(t, tt.encodings, accepted.encodings, tt.header)
		assert.Equal(t, tt.identity, accepted.identity, tt.header)
	}
}
