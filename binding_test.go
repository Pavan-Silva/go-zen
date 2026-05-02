package zen

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test ProtoBuf binding (simplified - requires proper proto message)
// Commented out because it requires a proper proto.Message implementation
/*
type TestMessage struct {
	Id   int32  `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (m *TestMessage) Reset()         {}
func (m *TestMessage) String() string { return proto.CompactTextString(m) }
func (m *TestMessage) ProtoMessage()  {}

func TestProtoBuf_Bind(t *testing.T) {
	r := New(":0")
	r.Handle("POST /proto", func(c *Context) {
		var msg TestMessage
		if err := c.BindProtoBuf(&msg); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, msg)
	})

	msg := &TestMessage{Id: 1, Name: "test"}
	data, _ := proto.Marshal(msg)
	req := httptest.NewRequest("POST", "/proto", nil)
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestProtoBufResponse(t *testing.T) {
	r := New(":0")
	r.Handle("GET /proto", func(c *Context) {
		msg := &TestMessage{Id: 42, Name: "response"}
		c.ProtoBuf(http.StatusOK, msg)
	})

	req := httptest.NewRequest("GET", "/proto", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/x-protobuf" {
		t.Fatalf("Content-Type = %q, want application/x-protobuf", w.Header().Get("Content-Type"))
	}
}
*/

// Test Header binding
func TestBindHeader(t *testing.T) {
	type Headers struct {
		UserID string `header:"X-User-Id"`
		APIKey string `header:"X-Api-Key"`
		Rate   int    `header:"X-Rate-Limit"`
	}

	r := New(":0")
	r.Handle("GET /headers", func(c *Context) {
		var h Headers
		if err := c.BindHeader(&h); err != nil {
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
	r.Handle("GET /headers", func(c *Context) {
		var h Headers
		if err := c.BindHeader(&h); err != nil {
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
	r.Handle("GET /headers", func(c *Context) {
		// Pass a non-pointer (invalid dest)
		var h struct{}
		if err := c.BindHeader(h); err != nil {
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
