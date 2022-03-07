package utils

import (
	"io"
	"net/http"
	"net/http/httptest"
)

type Header struct {
	Key   string
	Value string
}

// PerformRequest - Mock Go Gin HTTP test requests
func PerformRequest(r http.Handler, method, path string, body io.Reader, headers ...Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
