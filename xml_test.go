package zen

import (
	"encoding/xml"
	"net/http/httptest"
	"strings"
	"testing"
)

type testXMLUser struct {
	XMLName xml.Name `xml:"user"`
	Name    string   `xml:"name"`
	Email   string   `xml:"email"`
	Age     int      `xml:"age"`
}

func TestBindXML_Valid(t *testing.T) {
	r := New(":0")
	var captured testXMLUser
	r.Handle("POST /user", func(c *Context) {
		if err := c.BindXML(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.XML(200, captured)
	})

	body := strings.NewReader(`<user><name>John</name><email>john@example.com</email><age>30</age></user>`)
	req := httptest.NewRequest("POST", "/user", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if captured.Name != "John" {
		t.Fatalf("name = %q, want %q", captured.Name, "John")
	}
}

func TestBindXML_Malformed(t *testing.T) {
	r := New(":0")
	var captured testXMLUser
	r.Handle("POST /user", func(c *Context) {
		if err := c.BindXML(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := strings.NewReader(`<user><name>John</name`)
	req := httptest.NewRequest("POST", "/user", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBindXML_TrailingData(t *testing.T) {
	r := New(":0")
	var captured testXMLUser
	r.Handle("POST /user", func(c *Context) {
		if err := c.BindXML(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := strings.NewReader(`<user><name>John</name></user><user><name>Jane</name></user>`)
	req := httptest.NewRequest("POST", "/user", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "only one XML") {
		t.Fatalf("error message should mention single element; got: %s", w.Body.String())
	}
}

func TestBindXML_NonStruct(t *testing.T) {
	r := New(":0")
	type container struct {
		Value string `xml:"value"`
	}
	var captured container
	r.Handle("POST /xml", func(c *Context) {
		if err := c.BindXML(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.XML(200, captured)
	})

	body := strings.NewReader(`<container><value>hello</value></container>`)
	req := httptest.NewRequest("POST", "/xml", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured.Value != "hello" {
		t.Fatalf("value = %q, want %q", captured.Value, "hello")
	}
}

func TestXML_Response(t *testing.T) {
	r := New(":0")
	r.Handle("GET /xml", func(c *Context) {
		c.XML(200, testXMLUser{Name: "John", Email: "john@example.com", Age: 30})
	})

	req := httptest.NewRequest("GET", "/xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/xml")
	}
	if !strings.Contains(w.Body.String(), "<user>") {
		t.Fatalf("body should contain <user> tag; got: %s", w.Body.String())
	}
}

func TestXML_EncodeError(t *testing.T) {
	r := New(":0")
	r.Handle("GET /bad", func(c *Context) {
		c.XML(200, make(chan int))
	})

	req := httptest.NewRequest("GET", "/bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// With delayedStatusWriter, the status can be changed to 500 on encode error.
	if w.Code != 500 {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<error>") {
		t.Fatalf("XML error response should contain <error> tag; got: %s", w.Body.String())
	}
}

func BenchmarkBindXML(b *testing.B) {
	r := New(":0")
	var captured testXMLUser
	r.Handle("POST /user", func(c *Context) {
		c.BindXML(&captured)
		c.String(200, "ok")
	})

	body := strings.NewReader(`<user><name>John</name><email>john@example.com</email><age>30</age></user>`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/user", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkXML(b *testing.B) {
	r := New(":0")
	r.Handle("GET /xml", func(c *Context) {
		c.XML(200, testXMLUser{Name: "John", Email: "john@example.com", Age: 30})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/xml", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
