package api

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ziggornif/gimme/internal/content"
)

// acceptedEncodings is the outcome of Accept-Encoding negotiation: the stored
// encodings the client accepts, in preference order, and whether the unencoded
// representation may be served.
type acceptedEncodings struct {
	encodings []content.Encoding
	identity  bool
}

// parseAcceptEncoding applies the acceptability rules of RFC 9110 section 12.5.3:
// a listed coding is acceptable unless its qvalue is 0, "*" covers the codings no
// entry names, and the unencoded representation stays acceptable unless refused by
// "identity;q=0" or by a "*;q=0" that no identity entry overrides.
func parseAcceptEncoding(header string) acceptedEncodings {
	if strings.TrimSpace(header) == "" {
		return acceptedEncodings{identity: true}
	}

	qualities := map[string]float64{}
	for _, part := range strings.Split(header, ",") {
		segments := strings.Split(strings.TrimSpace(part), ";")
		token := strings.ToLower(strings.TrimSpace(segments[0]))
		quality := 1.0
		valid := true
		for _, parameter := range segments[1:] {
			name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !strings.EqualFold(name, "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if !found || err != nil || math.IsNaN(parsed) || parsed < 0 || parsed > 1 {
				valid = false
				break
			}
			quality = parsed
		}
		if !valid || token == "" {
			continue
		}

		if current, exists := qualities[token]; exists && quality <= current {
			continue
		}
		qualities[token] = quality
	}

	wildcard, hasWildcard := qualities["*"]

	resolve := func(token string) (float64, bool) {
		if named, exists := qualities[token]; exists {
			return named, true
		}
		if hasWildcard {
			return wildcard, true
		}
		return 0, false
	}

	identityQuality, identityListed := resolve("identity")
	accepted := acceptedEncodings{identity: !identityListed || identityQuality > 0}

	type preference struct {
		encoding content.Encoding
		quality  float64
	}
	var preferences []preference
	for _, encoding := range []content.Encoding{content.EncodingBrotli, content.EncodingGzip} {
		quality, listed := resolve(string(encoding))
		if !listed || quality == 0 {
			continue
		}
		preferences = append(preferences, preference{encoding: encoding, quality: quality})
	}
	if len(preferences) == 0 {
		return accepted
	}

	sort.SliceStable(preferences, func(i, j int) bool {
		return preferences[i].quality > preferences[j].quality
	})
	accepted.encodings = make([]content.Encoding, len(preferences))
	for i, preference := range preferences {
		accepted.encodings[i] = preference.encoding
	}
	return accepted
}
