package zen

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestEngine() *Engine {
	return New(":0")
}

func TestValidation_Required(t *testing.T) {
	e := newTestEngine()

	type S struct {
		Name string `validate:"required"`
	}

	s := S{Name: ""}
	err := e.Validate(&s)
	if err == nil {
		t.Fatal("expected validation error for empty required field")
	}

	s.Name = "John"
	err = e.Validate(&s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_Email(t *testing.T) {
	e := newTestEngine()

	type S struct {
		Email string `validate:"email"`
	}

	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"test.name@domain.co", true},
		{"invalid", false},
		{"@missing.com", false},
		{"no-at-sign", false},
	}

	for _, tt := range tests {
		s := S{Email: tt.email}
		err := e.Validate(&s)
		if tt.valid && err != nil {
			t.Errorf("email %q should be valid, got error: %v", tt.email, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("email %q should be invalid", tt.email)
		}
	}
}

func TestValidation_GTE(t *testing.T) {
	e := newTestEngine()

	type S struct {
		Age int `validate:"gte=18"`
	}

	tests := []struct {
		age   int
		valid bool
	}{
		{17, false},
		{18, true},
		{25, true},
	}

	for _, tt := range tests {
		s := S{Age: tt.age}
		err := e.Validate(&s)
		if tt.valid && err != nil {
			t.Errorf("age %d should be valid, got error: %v", tt.age, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("age %d should be invalid", tt.age)
		}
	}
}

func TestValidate_NonStruct(t *testing.T) {
	e := newTestEngine()

	m := map[string]any{"key": "value"}
	err := e.Validate(&m)
	if err != nil {
		t.Fatalf("non-struct should not error: %v", err)
	}
}

func TestValidate_PointerToStruct(t *testing.T) {
	e := newTestEngine()

	type S struct {
		Name string `validate:"required"`
	}

	err := e.Validate(&S{Name: "test"})
	if err != nil {
		t.Fatalf("valid struct should pass: %v", err)
	}

	err = e.Validate(&S{Name: ""})
	if err == nil {
		t.Fatal("empty required field should fail")
	}
}

func TestValidate_DirectStruct(t *testing.T) {
	e := newTestEngine()

	type S struct {
		Name string `validate:"required"`
	}

	err := e.Validate(S{Name: "test"})
	if err != nil {
		t.Fatalf("valid struct should pass: %v", err)
	}
}

func TestSetValidator_Custom(t *testing.T) {
	e := newTestEngine()

	called := false
	v := Validator(ValidatorFunc(func(i any) error {
		called = true
		return nil
	}))
	e.SetValidator(v)

	type S struct {
		Name string
	}
	err := e.Validate(&S{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("custom validator was not called")
	}
}

func TestSetValidator_Nil(t *testing.T) {
	e := newTestEngine()
	e.SetValidator(nil)

	type S struct {
		Name string `validate:"required"`
	}
	err := e.Validate(&S{Name: ""})
	if err != nil {
		t.Fatal("nil validator should not validate")
	}
}

func TestEnableAutoValidation(t *testing.T) {
	e := New(":0")
	e.EnableAutoValidation()

	e.POST("/test", func(c *Ctx) {
		var v struct {
			Name string `validate:"required"`
		}
		if err := c.BindJSON(&v); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := strings.NewReader(`{"name":""}`)
	req := httptest.NewRequest("POST", "/test", body)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("auto-validation enabled: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func BenchmarkValidation(b *testing.B) {
	e := newTestEngine()

	type S struct {
		Name  string `validate:"required"`
		Email string `validate:"email"`
		Age   int    `validate:"gte=0"`
	}

	s := S{Name: "John", Email: "john@example.com", Age: 30}

	b.ReportAllocs()
	for b.Loop() {
		_ = e.Validate(&s)
	}
}
