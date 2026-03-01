package zen_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pavan-Silva/zen/zen"
	"github.com/go-playground/validator/v10"
)

func newContext(method, target, body string, contentType string) (*zen.Context, *httptest.ResponseRecorder) {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, bodyReader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	return &zen.Context{Response: w, Request: req}, w
}

func TestJSONRoundTrip(t *testing.T) {
	c, _ := newContext("POST", "/", `{"foo":"bar"}`, "application/json")
	var data map[string]string
	if err := c.BindJSON(&data); err != nil {
		t.Fatal(err)
	}
	if data["foo"] != "bar" {
		t.Errorf("expected bar got %s", data["foo"])
	}

	c2, w2 := newContext("GET", "/", "", "")
	c2.JSON(http.StatusOK, data)
	if w2.Code != http.StatusOK {
		t.Errorf("status %d", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "bar") {
		t.Errorf("body missing bar: %s", w2.Body.String())
	}
}

func TestBindJSONValidation(t *testing.T) {
	type payload struct {
		Username string `json:"username" binding:"required,alphanum"`
		Email    string `json:"email"    binding:"required,email"`
	}

	// valid payload — should pass
	c, _ := newContext("POST", "/", `{"username":"alice","email":"alice@example.com"}`, "application/json")
	var p payload
	if err := c.BindJSON(&p); err != nil {
		t.Errorf("valid payload should pass, got %v", err)
	}

	// invalid payload — should return validator.ValidationErrors
	c2, _ := newContext("POST", "/", `{"username":"","email":"not-an-email"}`, "application/json")
	var p2 payload
	err := c2.BindJSON(&p2)
	if err == nil {
		t.Fatal("invalid payload should fail")
	}
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		t.Errorf("expected validator.ValidationErrors, got %T: %v", err, err)
	}

	// malformed JSON — should return a decode error, not ValidationErrors
	c3, _ := newContext("POST", "/", `{bad json}`, "application/json")
	var p3 payload
	err = c3.BindJSON(&p3)
	if err == nil {
		t.Fatal("malformed JSON should fail")
	}
	var ve2 validator.ValidationErrors
	if errors.As(err, &ve2) {
		t.Errorf("malformed JSON should not return ValidationErrors")
	}
}

func TestFormBind(t *testing.T) {
	type payload struct {
		Foo string `json:"foo" binding:"required"`
		Baz string `json:"baz" binding:"required"`
	}

	// valid form
	c, _ := newContext("POST", "/", "foo=bar&baz=qux", "application/x-www-form-urlencoded")
	var dest payload
	if err := c.BindForm(&dest); err != nil {
		t.Fatal(err)
	}
	if dest.Foo != "bar" || dest.Baz != "qux" {
		t.Errorf("unexpected values %+v", dest)
	}

	// missing required field — should return ValidationErrors
	c2, _ := newContext("POST", "/", "foo=bar", "application/x-www-form-urlencoded")
	var dest2 payload
	err := c2.BindForm(&dest2)
	if err == nil {
		t.Fatal("missing required field should fail")
	}
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		t.Errorf("expected validator.ValidationErrors, got %T: %v", err, err)
	}
}

func TestBody(t *testing.T) {
	c, _ := newContext("POST", "/", "hello", "")
	b, err := c.Body()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Errorf("expected hello got %s", string(b))
	}
}

func TestQueryParam(t *testing.T) {
	c, _ := newContext("GET", "/?name=zen&version=1", "", "")
	if c.QueryParam("name") != "zen" {
		t.Errorf("expected zen got %s", c.QueryParam("name"))
	}
	if c.QueryParam("version") != "1" {
		t.Errorf("expected 1 got %s", c.QueryParam("version"))
	}
	if c.QueryParam("missing") != "" {
		t.Errorf("expected empty string for missing key")
	}
}

func TestJSONSetsContentType(t *testing.T) {
	c, w := newContext("GET", "/", "", "")
	c.JSON(http.StatusOK, map[string]string{"k": "v"})
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json got %s", ct)
	}
}

func TestJSONDoesNotOverwriteContentType(t *testing.T) {
	c, w := newContext("GET", "/", "", "")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.JSON(http.StatusOK, map[string]string{"k": "v"})
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("content type should not be overwritten")
	}
}
