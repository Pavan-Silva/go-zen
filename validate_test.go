package zen

import (
	"testing"
)

func TestValidatorInstance(t *testing.T) {
	v := validatorInstance()
	if v == nil {
		t.Fatal("validatorInstance returned nil")
	}
}

func TestValidation_Required(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}

	s := S{Name: ""}
	err := validatorInstance().Struct(s)
	if err == nil {
		t.Fatal("expected validation error for empty required field")
	}

	s.Name = "John"
	err = validatorInstance().Struct(s)
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
		err := validatorInstance().Struct(s)
		if tt.valid && err != nil {
			t.Errorf("email %q should be valid, got error: %v", tt.email, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("email %q should be invalid", tt.email)
		}
	}
}

func TestValidation_EmailStrict(t *testing.T) {
	type S struct {
		Email string `validate:"email-strict"`
	}

	s := S{Email: "user@example.com"}
	err := validatorInstance().Struct(s)
	if err != nil {
		t.Fatalf("valid email should pass strict validation: %v", err)
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
		err := validatorInstance().Struct(s)
		if tt.valid && err != nil {
			t.Errorf("age %d should be valid, got error: %v", tt.age, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("age %d should be invalid", tt.age)
		}
	}
}

func TestValidateStruct_NonStruct(t *testing.T) {
	m := map[string]any{"key": "value"}
	err := validateStruct(&m)
	if err != nil {
		t.Fatalf("non-struct should not error: %v", err)
	}
}

func TestValidateStruct_PointerToStruct(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}

	err := validateStruct(&S{Name: "test"})
	if err != nil {
		t.Fatalf("valid struct should pass: %v", err)
	}

	err = validateStruct(&S{Name: ""})
	if err == nil {
		t.Fatal("empty required field should fail")
	}
}

func TestValidateStruct_DirectStruct(t *testing.T) {
	type S struct {
		Name string `validate:"required"`
	}

	err := validateStruct(S{Name: "test"})
	if err != nil {
		t.Fatalf("valid struct should pass: %v", err)
	}
}

func TestEmailRegex(t *testing.T) {
	if !emailRegex.MatchString("user@example.com") {
		t.Fatal("emailRegex should match user@example.com")
	}
	if emailRegex.MatchString("invalid") {
		t.Fatal("emailRegex should not match invalid")
	}
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
	for i := 0; i < b.N; i++ {
		validatorInstance().Struct(s)
	}
}
