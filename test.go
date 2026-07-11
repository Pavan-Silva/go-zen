package zen

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
)

// TestRequest bundles a request and response recorder for convenient testing.
type TestRequest struct {
	Request  *http.Request
	Response *httptest.ResponseRecorder
}

// NewTestRequest creates a TestRequest with the given method, path, and optional body.
//
// Example:
//
//	tr := zen.NewTestRequest("GET", "/api/users", nil)
//	r.ServeHTTP(tr.Response, tr.Request)
//	fmt.Println(tr.Response.Code)
func NewTestRequest(method, path string, body io.Reader) *TestRequest {
	return &TestRequest{
		Request:  httptest.NewRequest(method, path, body),
		Response: httptest.NewRecorder(),
	}
}

// NewTestRequestWithJSON creates a test request with a JSON string body and sets
// the Content-Type header to application/json.
//
// Example:
//
//	tr := zen.NewTestRequestWithJSON("POST", "/api/users", `{"name":"John"}`)
//	r.ServeHTTP(tr.Response, tr.Request)
func NewTestRequestWithJSON(method, path, jsonBody string) *TestRequest {
	body := bytes.NewBufferString(jsonBody)
	tr := NewTestRequest(method, path, body)
	tr.Request.Header.Set("Content-Type", "application/json")
	return tr
}

// Serve calls Engine.ServeHTTP with the test request.
func (tr *TestRequest) Serve(r *Engine) {
	r.ServeHTTP(tr.Response, tr.Request)
}
