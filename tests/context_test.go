package zen_test

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/Pavan-Silva/zen/zen"
)

func TestJSONRoundTrip(t *testing.T) {
    req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"foo":"bar"}`))
    w := httptest.NewRecorder()

    c := &zen.Context{Response: w, Request: req}

    var data map[string]string
    if err := c.BindJSON(&data); err != nil {
        t.Fatal(err)
    }
    if data["foo"] != "bar" {
        t.Errorf("expected bar got %s", data["foo"])
    }

    if err := c.JSON(http.StatusOK, data); err != nil {
        t.Fatal(err)
    }
    if w.Code != http.StatusOK {
        t.Errorf("status %d", w.Code)
    }
    body := w.Body.String()
    if !strings.Contains(body, "bar") {
        t.Errorf("body missing bar: %s", body)
    }
}

func TestFormBind(t *testing.T) {
    form := "foo=bar&baz=qux"
    req := httptest.NewRequest("POST", "/", strings.NewReader(form))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    w := httptest.NewRecorder()

    c := &zen.Context{Response: w, Request: req}
    var dest struct{
        Foo string `json:"foo"`
        Baz string `json:"baz"`
    }
    if err := c.BindForm(&dest); err != nil {
        t.Fatal(err)
    }
    if dest.Foo != "bar" || dest.Baz != "qux" {
        t.Errorf("unexpected values %+v", dest)
    }
}

func TestBody(t *testing.T) {
    content := "hello" 
    req := httptest.NewRequest("POST", "/", strings.NewReader(content))
    c := &zen.Context{Response: httptest.NewRecorder(), Request: req}
    b, err := c.Body()
    if err != nil {
        t.Fatal(err)
    }
    if string(b) != content {
        t.Errorf("expected %s got %s", content, string(b))
    }
}

func TestValidate(t *testing.T) {
    type input struct {
        Name  string `validate:"required"`
        Email string `validate:"required,email"`
    }
    var good = input{Name: "foo", Email: "foo@bar.com"}
    var bad = input{Name: "", Email: "not an email"}

    c := &zen.Context{}
    if err := c.Validate(good); err != nil {
        t.Errorf("valid struct should pass, got %v", err)
    }
    if err := c.Validate(bad); err == nil {
        t.Errorf("invalid struct unexpectedly passed")
    }
}
