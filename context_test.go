package zen

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestContext_Set_Get_InlineSlots(t *testing.T) {
	c := &Context{
		overflow: make(map[string]any, 4),
	}

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

func TestContext_Set_Get_Overflow(t *testing.T) {
	c := &Context{
		overflow: make(map[string]any, 4),
	}

	for i := 0; i < 12; i++ {
		c.Set(fmt.Sprintf("k%d", i), i)
	}

	for i := 0; i < 12; i++ {
		got := c.Get(fmt.Sprintf("k%d", i))
		if got != i {
			t.Fatalf("k%d = %v, want %d", i, got, i)
		}
	}
}

func TestContext_Set_UpdatesExisting(t *testing.T) {
	c := &Context{
		overflow: make(map[string]any, 4),
	}

	c.Set("foo", "bar")
	c.Set("foo", "baz")

	if got := c.Get("foo"); got != "baz" {
		t.Fatalf("foo = %v, want baz", got)
	}
}

func TestContext_Set_OverflowUpdatesExisting(t *testing.T) {
	c := &Context{
		overflow: make(map[string]any, 4),
	}

	for i := 0; i < 10; i++ {
		c.Set(fmt.Sprintf("k%d", i), i)
	}
	c.Set("k5", 999)

	if got := c.Get("k5"); got != 999 {
		t.Fatalf("k5 = %v, want 999", got)
	}
}

func TestContext_Reset(t *testing.T) {
	c := &Context{
		overflow: make(map[string]any, 4),
	}

	c.Set("a", 1)
	c.Set("b", 2)

	c.reset(nil, nil)

	if c.storeLen != 0 {
		t.Fatalf("storeLen = %d, want 0", c.storeLen)
	}
	if len(c.overflow) != 0 {
		t.Fatalf("overflow len = %d, want 0", len(c.overflow))
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

func TestContext_InlineSlotsCapacity(t *testing.T) {
	if inlineSlots != 8 {
		t.Fatalf("inlineSlots = %d, want 8", inlineSlots)
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
