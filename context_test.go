package zen

import (
	"net/http/httptest"
	"testing"
)

func TestContext_Set_Get(t *testing.T) {
	c := &Context{}

	c.Set("user_id", 42)
	c.Set("request_id", "abc-123")
	c.Set("flag", true)

	if got := c.Get("user_id"); got != 42 {
		t.Fatalf("user_id = %v, want 42", got)
	}
	if got := c.Get("request_id"); got != "abc-123" {
		t.Fatalf("request_id = %v, want abc-123", got)
	}
	if got := c.Get("flag"); got != true {
		t.Fatalf("flag = %v, want true", got)
	}
	if got := c.Get("missing"); got != nil {
		t.Fatalf("missing = %v, want nil", got)
	}
}

func TestContext_Set_UpdatesExisting(t *testing.T) {
	c := &Context{}

	c.Set("foo", "bar")
	c.Set("foo", "baz")

	if got := c.Get("foo"); got != "baz" {
		t.Fatalf("foo = %v, want baz", got)
	}
}

func TestContext_Reset(t *testing.T) {
	c := &Context{}

	c.Set("a", 1)
	c.Set("b", 2)

	c.reset(nil, nil)

	if c.Get("a") != nil {
		t.Fatal("a should be nil after reset")
	}
	if c.Get("b") != nil {
		t.Fatal("b should be nil after reset")
	}
}

func TestContext_FromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if c := FromRequest(r); c != nil {
		t.Fatal("FromRequest should return nil for raw request")
	}

	w := httptest.NewRecorder()
	c, r := newContext(w, r)
	defer releaseContext(c)

	got := FromRequest(r)
	if got != c {
		t.Fatal("FromRequest should return the attached context")
	}
}

func TestContextPool_Reuse(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	c1, r1 := newContext(w, r)
	c1.Set("temp", "value")
	releaseContext(c1)

	c2, _ := newContext(w, r1)
	defer releaseContext(c2)

	if c2.Get("temp") != nil {
		t.Fatal("pooled context should not leak data from previous request")
	}
}

func TestContext_RequestResponseSet(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	c, r := newContext(w, r)
	defer releaseContext(c)

	if c.Response != w {
		t.Fatal("Response not set correctly")
	}
	if c.Request != r {
		t.Fatal("Request not set correctly")
	}
}


