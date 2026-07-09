package zen

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultValidator(t *testing.T) {
	if getValidator() == nil {
		t.Fatal("defaultValidator should not be nil")
	}
}

func TestValidation_Required(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}

	s := S{Name: ""}
	err := Validate(&s)
	if err == nil {
		t.Fatal("expected validation error for empty required field")
	}

	s.Name = "John"
	err = Validate(&s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_Email(t *testing.T) {
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
		err := Validate(&s)
		if tt.valid && err != nil {
			t.Errorf("email %q should be valid, got error: %v", tt.email, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("email %q should be invalid", tt.email)
		}
	}
}

func TestValidation_GTE(t *testing.T) {
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
		err := Validate(&s)
		if tt.valid && err != nil {
			t.Errorf("age %d should be valid, got error: %v", tt.age, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("age %d should be invalid", tt.age)
		}
	}
}

func TestValidate_NonStruct(t *testing.T) {
	m := map[string]any{"key": "value"}
	err := Validate(&m)
	if err != nil {
		t.Fatalf("non-struct should not error: %v", err)
	}
}

func TestValidate_PointerToStruct(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}

	err := Validate(&S{Name: "test"})
	if err != nil {
		t.Fatalf("valid struct should pass: %v", err)
	}

	err = Validate(&S{Name: ""})
	if err == nil {
		t.Fatal("empty required field should fail")
	}
}

func TestValidate_DirectStruct(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}

	err := Validate(S{Name: "test"})
	if err != nil {
		t.Fatalf("valid struct should pass: %v", err)
	}
}

func TestSetValidator_Custom(t *testing.T) {
	original := getValidator()
	t.Cleanup(func() { SetValidator(original) })

	called := false
	SetValidator(ValidatorFunc(func(i any) error {
		called = true
		return nil
	}))

	type S struct {
		Name string
	}
	err := Validate(&S{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("custom validator was not called")
	}
}

func TestSetValidator_Nil(t *testing.T) {
	original := getValidator()
	t.Cleanup(func() { SetValidator(original) })

	SetValidator(nil)

	type S struct {
		Name string `validate:"required"`
	}
	err := Validate(&S{Name: ""})
	if err != nil {
		t.Fatal("nil validator should not validate")
	}
}

func TestEnableAutoValidation(t *testing.T) {
	EnableAutoValidation()
	defer func() {
		autoValidateMu.Lock()
		autoValidateEnabled = false
		autoValidateMu.Unlock()
	}()

	r := New(":0")
	r.POST("/test", func(c *Ctx) {
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
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("auto-validation enabled: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

type ValidatorFunc func(i any) error

func (f ValidatorFunc) Validate(i any) error {
	return f(i)
}

func BenchmarkValidation(b *testing.B) {
	type S struct {
		Name  string `validate:"required"`
		Email string `validate:"email"`
		Age   int    `validate:"gte=0"`
	}

	s := S{Name: "John", Email: "john@example.com", Age: 30}

	b.ReportAllocs()
	b.ResetTimer()
	_ = getValidator()
	for i := 0; i < b.N; i++ {
		_ = Validate(&s)
	}
}
