package zen

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test Header binding
func TestBindHeader(t *testing.T) {
	type Headers struct {
		UserID string `header:"X-User-Id"`
		APIKey string `header:"X-Api-Key"`
		Rate   int    `header:"X-Rate-Limit"`
	}

	r := New(":0")
	r.GET("/headers", func(c *Ctx) {
		var h Headers
		if err := BindHeaders(c, &h); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, h)
	})

	req := httptest.NewRequest("GET", "/headers", nil)
	req.Header.Set("X-User-Id", "user-123")
	req.Header.Set("X-Api-Key", "secret-key")
	req.Header.Set("X-Rate-Limit", "100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() == "" {
		t.Fatal("body should not be empty")
	}
}

func TestBindHeader_NotFound(t *testing.T) {
	type Headers struct {
		UserID string `header:"X-User-Id"`
	}

	r := New(":0")
	r.GET("/headers", func(c *Ctx) {
		var h Headers
		if err := BindHeaders(c, &h); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, h)
	})

	req := httptest.NewRequest("GET", "/headers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBindHeader_InvalidDest(t *testing.T) {
	r := New(":0")
	r.GET("/headers", func(c *Ctx) {
		// Pass a non-pointer (invalid dest)
		var h struct{}
		if err := BindHeaders(c, h); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/headers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// 1. Test single-stage validation flow inside c.Bind (prevent early premature failure)
func TestBind_ValidationSingleStage(t *testing.T) {
	type Request struct {
		ID   string `param:"id" validate:"required"`
		Name string `json:"name" validate:"required"`
	}

	r := New(":0")
	r.EnableAutoValidation() // Enable auto-validation

	r.POST("/items/{id}", func(c *Ctx) {
		var req Request
		if err := c.Bind(&req); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, req)
	})

	body := `{"name":"John"}`
	req := httptest.NewRequest("POST", "/items/123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// 2. Test tag fallbacks: source tag -> json tag -> field name
func TestBind_TagFallbacks(t *testing.T) {
	type Request struct {
		ParamVal string `param:"param_val"`
		JSONVal  string `json:"json_val"`
		Untagged string
	}

	r := New(":0")

	r.GET("/test/{param_val}", func(c *Ctx) {
		var req Request
		if err := c.Bind(&req); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, req)
	})

	req := httptest.NewRequest("GET", "/test/hello?json_val=world&Untagged=yes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	actual := strings.TrimSpace(w.Body.String())
	if !strings.Contains(actual, `"ParamVal":"hello"`) ||
		!strings.Contains(actual, `"json_val":"world"`) ||
		!strings.Contains(actual, `"Untagged":"yes"`) {
		t.Fatalf("unexpected body = %s, expected to contain param, json-tag, and field-name fallback values", actual)
	}
}

// 2b. Test json:"-" fields still bind via field name fallback
func TestBind_TagFallback_JSONDash(t *testing.T) {
	type Request struct {
		JSONIgnored string `json:"-"`
	}

	r := New(":0")
	var captured Request
	r.GET("/test", func(c *Ctx) {
		if err := BindQueryParams(c, &captured); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test?JSONIgnored=bound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if captured.JSONIgnored != "bound" {
		t.Fatalf("JSONIgnored = %q, want %q", captured.JSONIgnored, "bound")
	}
}

// 3. Test custom unmarshalers on slice elements
type CustomSliceInt int

func (c *CustomSliceInt) UnmarshalParam(param string) error {
	*c = CustomSliceInt(len(param)) // sets val to length of parameter
	return nil
}

func TestBind_SliceCustomElements(t *testing.T) {
	type Request struct {
		Ints []CustomSliceInt `query:"ints"`
	}

	r := New(":0")
	r.GET("/test", func(c *Ctx) {
		var req Request
		if err := BindQueryParams(c, &req); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, req.Ints)
	})

	req := httptest.NewRequest("GET", "/test?ints=abc&ints=xy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "[3,2]") {
		t.Fatalf("unexpected slice elements: %s", w.Body.String())
	}
}

// 4. Test map error handling: top-level map with unsupported key type
func TestBind_MapErrorHandling(t *testing.T) {
	dest := make(map[int]string)
	data := map[string][]string{"key": {"val"}}
	err := bindData(&dest, data, "query", nil)
	if err == nil {
		t.Fatal("expected error for map[int]string, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported map key type") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

// 4b. Test map error handling: unsupported value type
func TestBind_MapUnsupportedValueType(t *testing.T) {
	dest := make(map[string]int)
	data := map[string][]string{"key": {"val"}}
	err := bindData(&dest, data, "query", nil)
	if err == nil {
		t.Fatal("expected error for map[string]int, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported map value type") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

// 5. Test pluggable XML serializer
type CustomXMLSerializer struct{}

func (CustomXMLSerializer) Serialize(c *Ctx, v any) error {
	c.Response.Write([]byte("<custom>serialized</custom>"))
	return nil
}

func (CustomXMLSerializer) Deserialize(c *Ctx, v any) error {
	destStruct := v.(*struct {
		Val string `xml:"val"`
	})
	destStruct.Val = "plugged"
	return nil
}

func TestPluggableXMLSerializer(t *testing.T) {
	r := New(":0")
	r.XMLSerializer = CustomXMLSerializer{}

	r.POST("/xml", func(c *Ctx) {
		var req struct {
			Val string `xml:"val"`
		}
		if err := c.BindXML(&req); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		if req.Val != "plugged" {
			t.Errorf("expected Val to be 'plugged', got %q", req.Val)
		}
		c.XML(http.StatusOK, req)
	})

	body := `<req><val>hello</val></req>`
	req := httptest.NewRequest("POST", "/xml", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "<custom>serialized</custom>" {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

// 6. Test MaxMultipartMemory is respected in context methods
func TestFormFile_MaxMultipartMemory(t *testing.T) {
	r := New(":0")
	r.MaxMultipartMemory = 1 // 1 byte to force disk storage/check memory limits

	r.POST("/upload", func(c *Ctx) {
		_, _, err := c.FormFile("file")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	body := "--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\nhello\r\n--boundary--"
	req := httptest.NewRequest("POST", "/upload", strings.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// 7. Test nested recursive struct and pointer struct binding
func TestBind_RecursiveAndPointerStructs(t *testing.T) {
	type Address struct {
		City string `query:"city"`
	}
	type Profile struct {
		Age int `query:"age"`
	}
	type Request struct {
		Name string   `query:"name"`
		Addr Address  // Nesting struct with no tag
		Prof *Profile // Pointer to struct with no tag
	}

	r := New(":0")
	r.GET("/test", func(c *Ctx) {
		var req Request
		if err := BindQueryParams(c, &req); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, req)
	})

	req := httptest.NewRequest("GET", "/test?name=Alice&city=Paris&age=30", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	actual := w.Body.String()
	if !strings.Contains(actual, `"City":"Paris"`) || !strings.Contains(actual, `"Age":30`) || !strings.Contains(actual, `"Name":"Alice"`) {
		t.Fatalf("unexpected recursive bind result: %s", actual)
	}
}

// 8. Test Header binding using canonical key optimization
func TestBind_HeaderCanonicalOptimization(t *testing.T) {
	type Request struct {
		CustomHeader string `header:"X-Custom-Header"`
		LowerHeader  string `header:"x-lower-header"`
	}

	r := New(":0")
	r.GET("/headers", func(c *Ctx) {
		var req Request
		if err := BindHeaders(c, &req); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, req)
	})

	req := httptest.NewRequest("GET", "/headers", nil)
	req.Header.Set("X-Custom-Header", "canonical")
	req.Header.Set("X-Lower-Header", "lowercase")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	actual := w.Body.String()
	if !strings.Contains(actual, `"CustomHeader":"canonical"`) || !strings.Contains(actual, `"LowerHeader":"lowercase"`) {
		t.Fatalf("unexpected header bind result: %s", actual)
	}
}

// A self-referencing struct must return a depth error instead of overflowing the stack.
func TestBind_SelfReferencingStructReturnsError(t *testing.T) {
	type Node struct {
		Name string `query:"name"`
		Next *Node  `query:"next"`
	}

	r := New(":0")
	r.GET("/test", func(c *Ctx) {
		var node Node
		if err := BindQueryParams(c, &node); err != nil {
			c.String(http.StatusBadRequest, "bind error: "+err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test?next=child&name=root", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "recursion depth") {
		t.Fatalf("body = %q, want a recursion depth error", w.Body.String())
	}
}

// Binding into a struct with an unexported struct-typed field must skip the
// field instead of panicking on reflect.Value.Interface.
func TestBind_UnexportedStructFieldSkipped(t *testing.T) {
	type inner struct {
		X int `query:"x"`
	}
	type outer struct {
		state inner
		Name  string `query:"name"`
	}

	r := New(":0")
	r.GET("/test", func(c *Ctx) {
		var o outer
		if err := BindQueryParams(c, &o); err != nil {
			c.String(http.StatusBadRequest, "bind error: "+err.Error())
			return
		}
		if o.Name != "jane" {
			c.String(http.StatusInternalServerError, "name not bound")
			return
		}
		if o.state.X != 0 {
			c.String(http.StatusInternalServerError, "unexported field must be skipped")
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test?name=jane&x=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// A time.Time field must bind from a query value using the format tag.
func TestBind_TimeWithFormat(t *testing.T) {
	type req struct {
		When time.Time `query:"when" format:"2006-01-02"`
	}

	r := New(":0")
	r.GET("/test", func(c *Ctx) {
		var q req
		if err := BindQueryParams(c, &q); err != nil {
			c.String(http.StatusBadRequest, "bind error: "+err.Error())
			return
		}
		c.String(http.StatusOK, q.When.Format("2006-01-02"))
	})

	httpreq := httptest.NewRequest("GET", "/test?when=2024-03-15", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpreq)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "2024-03-15" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "2024-03-15")
	}
}
