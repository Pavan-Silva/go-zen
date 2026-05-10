package zen

import (
	"net/http/httptest"
	"sync"
	"testing"
)

func TestContext_Set_Get(t *testing.T) {
	c := &Context{}

	c.Set("user_id", 42)
	c.Set("request_id", "abc-123")
	c.Set("flag", true)

	if got, ok := c.Get("user_id"); !ok || got != 42 {
		t.Fatalf("user_id = %v, want 42", got)
	}
	if got, ok := c.Get("request_id"); !ok || got != "abc-123" {
		t.Fatalf("request_id = %v, want abc-123", got)
	}
	if got, ok := c.Get("flag"); !ok || got != true {
		t.Fatalf("flag = %v, want true", got)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("missing should not exist")
	}
}

func TestContext_Set_UpdatesExisting(t *testing.T) {
	c := &Context{}

	c.Set("foo", "bar")
	c.Set("foo", "baz")

	if got, ok := c.Get("foo"); !ok || got != "baz" {
		t.Fatalf("foo = %v, want baz", got)
	}
}

func TestContext_Reset(t *testing.T) {
	c := &Context{}

	c.Set("a", 1)
	c.Set("b", 2)

	c.reset(nil, nil)

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should not exist after reset")
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should not exist after reset")
	}
}

func TestContext_FromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if c, ok := FromRequest(r); ok {
		_ = c
		t.Fatal("FromRequest should return (nil, false) for raw request")
	}

	w := httptest.NewRecorder()
	c, r := newContext(w, r)
	defer releaseContext(c)

	got, ok := FromRequest(r)
	if !ok {
		t.Fatal("FromRequest should return (context, true) when attached")
	}
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

	if _, ok := c2.Get("temp"); ok {
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

func TestContext_ConcurrentAccess(t *testing.T) {
	c := &Context{}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "goroutine"
			c.Set(key, n)
			val, ok := c.Get(key)
			if !ok {
				t.Errorf("goroutine %d: key not found", n)
				return
			}
			if val.(int) != n {
				t.Errorf("goroutine %d: val = %d, want %d", n, val, n)
			}
		}(i)
	}
	wg.Wait()
}

func TestContext_GetNilMu(t *testing.T) {
	c := &Context{}
	_, ok := c.Get("any")
	if ok {
		t.Fatal("Get on context with nil mu should return false")
	}
}

func TestContext_GetNilStore(t *testing.T) {
	c := &Context{}
	c.Set("a", 1)

	// Reset store to nil but keep mu
	c.store = nil

	_, ok := c.Get("a")
	if ok {
		t.Fatal("Get with nil store should return false")
	}
}


