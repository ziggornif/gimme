package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

type Header struct {
	Key   string
	Value string
}

// PerformRequest - Mock Go Gin HTTP test requests
func PerformRequest(r http.Handler, method, path string, body string, headers ...Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
