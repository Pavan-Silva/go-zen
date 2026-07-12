package zen

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestContext_Set_Get(t *testing.T) {
	c := &Ctx{}

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
	c := &Ctx{}

	c.Set("foo", "bar")
	c.Set("foo", "baz")

	if got, ok := c.Get("foo"); !ok || got != "baz" {
		t.Fatalf("foo = %v, want baz", got)
	}
}

func TestContext_Reset(t *testing.T) {
	c := &Ctx{}

	c.Set("a", 1)
	c.Set("b", 2)

	c.reset(nil, nil, nil)

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should not exist after reset")
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should not exist after reset")
	}
}

func TestFromContext_NoCtxValue(t *testing.T) {
	ctx := context.Background()
	if c, ok := FromContext(ctx); ok {
		t.Fatal("FromContext with no value should return false")
	} else if c != nil {
		t.Fatal("FromContext with no value should return nil")
	}
}

func TestFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), zenCtxKey{}, "not a *Ctx")
	if c, ok := FromContext(ctx); ok {
		t.Fatal("FromContext with wrong type should return false")
	} else if c != nil {
		t.Fatal("FromContext with wrong type should return nil")
	}
}

func TestFromContext_Valid(t *testing.T) {
	expected := &Ctx{}
	ctx := context.WithValue(context.Background(), zenCtxKey{}, expected)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext should return true")
	}
	if got != expected {
		t.Fatal("FromContext should return the stored ctx")
	}
}

func TestFromContext_RoundTrip(t *testing.T) {
	original := &Ctx{Response: httptest.NewRecorder(), Request: httptest.NewRequest("GET", "/", nil)}
	original.Set("key", "value")

	ctx := context.WithValue(context.Background(), zenCtxKey{}, original)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext should return true")
	}
	if got.Response != original.Response {
		t.Fatal("Response not preserved")
	}
	if got.Request != original.Request {
		t.Fatal("Request not preserved")
	}
	v, ok := got.Get("key")
	if !ok || v != "value" {
		t.Fatal("store values not preserved")
	}
}

func TestContext_FromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if c, ok := FromRequest(r); ok {
		_ = c
		t.Fatal("FromRequest should return (nil, false) for raw request")
	}

	w := httptest.NewRecorder()
	c := &Ctx{Response: w, Request: r}
	req := r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))

	got, ok := FromRequest(req)
	if !ok {
		t.Fatal("FromRequest should return (ctx, true) when attached")
	}
	if got != c {
		t.Fatal("FromRequest should return the attached ctx")
	}
}

func TestContextPool_Reuse(t *testing.T) {
	e := New(":0")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	c1 := e.pool.Get().(*Ctx)
	c1.reset(w, r, e)
	c1.Set("temp", "value")
	c1.reset(nil, nil, e)
	e.pool.Put(c1)

	c2 := e.pool.Get().(*Ctx)
	c2.reset(w, r, e)
	if _, ok := c2.Get("temp"); ok {
		t.Fatal("pooled ctx should not leak data from previous request")
	}
	c2.reset(nil, nil, e)
	e.pool.Put(c2)
}

func TestContext_RequestResponseSet(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	c := &Ctx{Response: w, Request: r}
	if c.Response != w {
		t.Fatal("Response not set correctly")
	}
	if c.Request != r {
		t.Fatal("Request not set correctly")
	}
}

func TestContext_GetNilStore(t *testing.T) {
	c := &Ctx{}
	c.Set("a", 1)

	c.store = nil

	_, ok := c.Get("a")
	if ok {
		t.Fatal("Get with nil store should return false")
	}
}

func TestContext_ConcurrentAccess(t *testing.T) {
	c := &Ctx{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "goroutine_" + itoa(n)
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

func TestContext_Keys(t *testing.T) {
	c := &Ctx{}
	c.Set("a", 1)
	c.Set("b", "two")
	c.Set("c", true)

	keys := c.Keys()
	if keys == nil {
		t.Fatal("Keys() should not return nil")
	}
	if len(keys) != 3 {
		t.Fatalf("Keys() len = %d, want 3", len(keys))
	}
	if keys["a"] != 1 || keys["b"] != "two" || keys["c"] != true {
		t.Fatal("Keys() returned wrong values")
	}

	// modification to returned map should not affect original
	keys["a"] = 99
	if v, _ := c.Get("a"); v != 1 {
		t.Fatal("modifying Keys() map should not affect context")
	}
}

func TestContext_Keys_NilStore(t *testing.T) {
	c := &Ctx{}
	if keys := c.Keys(); keys != nil {
		t.Fatal("Keys() on empty context should return nil")
	}
}

func TestContext_Copy(t *testing.T) {
	c := &Ctx{}
	c.Set("a", 1)
	c.Set("b", "two")

	cp := c.Copy()
	if cp == c {
		t.Fatal("Copy() should return a new pointer")
	}

	v, ok := cp.Get("a")
	if !ok || v != 1 {
		t.Fatal("Copy() should preserve values")
	}
	v, ok = cp.Get("b")
	if !ok || v != "two" {
		t.Fatal("Copy() should preserve values")
	}

	// modify original, copy should be unaffected
	c.Set("a", 99)
	if v, _ := cp.Get("a"); v != 1 {
		t.Fatal("Copy() should be independent of original")
	}
}

func TestContext_Copy_NilStore(t *testing.T) {
	c := &Ctx{}
	cp := c.Copy()
	if cp == c {
		t.Fatal("Copy() should return a new pointer")
	}
	if _, ok := cp.Get("any"); ok {
		t.Fatal("Copy() of empty context should have empty store")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
