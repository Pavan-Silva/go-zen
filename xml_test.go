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
	r.POST("/user", func(c *Ctx) {
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
	r.POST("/user", func(c *Ctx) {
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
	r.POST("/user", func(c *Ctx) {
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

	// xml.Decoder doesn't fail on trailing data - it decodes first element
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (decoder ignores trailing data)", w.Code)
	}
}

func TestBindXML_NonStruct(t *testing.T) {
	r := New(":0")
	type container struct {
		Value string `xml:"value"`
	}
	var captured container
	r.POST("/xml", func(c *Ctx) {
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
	r.GET("/xml", func(c *Ctx) {
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
	r.GET("/bad", func(c *Ctx) {
		c.XML(200, make(chan int))
	})

	req := httptest.NewRequest("GET", "/bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// xml.Marshal fails on invalid types, error is logged
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (status already sent before encode error)", w.Code)
	}
}

func BenchmarkBindXML(b *testing.B) {
	r := New(":0")
	var captured testXMLUser
	r.POST("/user", func(c *Ctx) {
		_ = c.BindXML(&captured)
		c.String(200, "ok")
	})

	body := strings.NewReader(`<user><name>John</name><email>john@example.com</email><age>30</age></user>`)

	b.ReportAllocs()
	
	for b.Loop() {
		req := httptest.NewRequest("POST", "/user", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkXML(b *testing.B) {
	r := New(":0")
	r.GET("/xml", func(c *Ctx) {
		c.XML(200, testXMLUser{Name: "John", Email: "john@example.com", Age: 30})
	})

	b.ReportAllocs()
	
	for b.Loop() {
		req := httptest.NewRequest("GET", "/xml", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
