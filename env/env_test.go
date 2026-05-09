package env

import (
	"os"
	"testing"
	"time"
)

func TestGetString(t *testing.T) {
	_ = os.Setenv("TEST_STRING", "hello")
	defer os.Unsetenv("TEST_STRING")

	got := GetString("TEST_STRING", "default")
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestGetString_Default(t *testing.T) {
	got := GetString("NONEXISTENT_VAR_XYZ", "default")
	if got != "default" {
		t.Fatalf("got %q, want %q", got, "default")
	}
}

func TestGetInt(t *testing.T) {
	_ = os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	got := GetInt("TEST_INT", 0)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestGetInt_Default(t *testing.T) {
	got := GetInt("NONEXISTENT_VAR_XYZ", 99)
	if got != 99 {
		t.Fatalf("got %d, want 99", got)
	}
}

func TestGetInt_Invalid(t *testing.T) {
	_ = os.Setenv("TEST_INT_BAD", "not-a-number")
	defer os.Unsetenv("TEST_INT_BAD")

	got := GetInt("TEST_INT_BAD", 42)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestGetBool(t *testing.T) {
	_ = os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")

	got := GetBool("TEST_BOOL", false)
	if !got {
		t.Fatal("got false, want true")
	}
}

func TestGetBool_Default(t *testing.T) {
	got := GetBool("NONEXISTENT_VAR_XYZ", true)
	if !got {
		t.Fatal("got false, want true")
	}
}

func TestGetDuration(t *testing.T) {
	_ = os.Setenv("TEST_DURATION", "5s")
	defer os.Unsetenv("TEST_DURATION")

	got := GetDuration("TEST_DURATION", 0)
	if got != 5*time.Second {
		t.Fatalf("got %v, want %v", got, 5*time.Second)
	}
}

func TestGetDuration_Default(t *testing.T) {
	got := GetDuration("NONEXISTENT_VAR_XYZ", 10*time.Second)
	if got != 10*time.Second {
		t.Fatalf("got %v, want %v", got, 10*time.Second)
	}
}

func TestMustGetString(t *testing.T) {
	_ = os.Setenv("TEST_MUST_STRING", "value")
	defer os.Unsetenv("TEST_MUST_STRING")

	got := MustGetString("TEST_MUST_STRING")
	if got != "value" {
		t.Fatalf("got %q, want %q", got, "value")
	}
}

func TestMustGetString_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	MustGetString("NONEXISTENT_VAR_XYZ")
}

func TestMustGetInt(t *testing.T) {
	_ = os.Setenv("TEST_MUST_INT", "123")
	defer os.Unsetenv("TEST_MUST_INT")

	got := MustGetInt("TEST_MUST_INT")
	if got != 123 {
		t.Fatalf("got %d, want 123", got)
	}
}

func TestMustGetInt_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = os.Setenv("TEST_MUST_INT_BAD", "not-a-number")
	defer os.Unsetenv("TEST_MUST_INT_BAD")
	MustGetInt("TEST_MUST_INT_BAD")
}

func TestMustGetBool(t *testing.T) {
	_ = os.Setenv("TEST_MUST_BOOL", "true")
	defer os.Unsetenv("TEST_MUST_BOOL")

	got := MustGetBool("TEST_MUST_BOOL")
	if !got {
		t.Fatal("got false, want true")
	}
}

func TestMustGetBool_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = os.Setenv("TEST_MUST_BOOL_BAD", "not-a-bool")
	defer os.Unsetenv("TEST_MUST_BOOL_BAD")
	MustGetBool("TEST_MUST_BOOL_BAD")
}

func TestMustGetDuration(t *testing.T) {
	_ = os.Setenv("TEST_MUST_DURATION", "1m")
	defer os.Unsetenv("TEST_MUST_DURATION")

	got := MustGetDuration("TEST_MUST_DURATION")
	if got != 1*time.Minute {
		t.Fatalf("got %v, want %v", got, 1*time.Minute)
	}
}

func TestMustGetDuration_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = os.Setenv("TEST_MUST_DURATION_BAD", "not-a-duration")
	defer os.Unsetenv("TEST_MUST_DURATION_BAD")
	MustGetDuration("TEST_MUST_DURATION_BAD")
}
