package zen

import (
	"reflect"
	"testing"
)

func TestParseStructTag(t *testing.T) {
	type s struct {
		Explicit string `param:"id"`
		Fallback string `json:"name"`
		Skip     string `json:"-"`
		NoTag    string
		CommaOpt string `json:"field,omitempty"`
	}

	rt := reflect.TypeOf(s{})

	tests := []struct {
		field   string
		tagName string
		want    string
	}{
		{"Explicit", "param", "id"},
		{"Explicit", "json", "Explicit"},
		{"Fallback", "query", "name"},
		{"Skip", "query", "Skip"},
		{"NoTag", "param", "NoTag"},
		{"CommaOpt", "json", "field"},
	}

	for _, tt := range tests {
		f, _ := rt.FieldByName(tt.field)
		got := parseStructTag(f, tt.tagName)
		if got != tt.want {
			t.Errorf("parseStructTag(%q, %q) = %q, want %q", tt.field, tt.tagName, got, tt.want)
		}
	}
}

func TestSetFieldValue_String(t *testing.T) {
	var s string
	fv := reflect.ValueOf(&s).Elem()
	if err := setFieldValue(fv, reflect.String, []string{"hello"}); err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Fatalf("got %q, want %q", s, "hello")
	}
}

func TestSetFieldValue_Int(t *testing.T) {
	var n int64
	fv := reflect.ValueOf(&n).Elem()
	if err := setFieldValue(fv, reflect.Int64, []string{"42"}); err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Fatalf("got %d, want %d", n, 42)
	}
}

func TestSetFieldValue_Int_Error(t *testing.T) {
	var n int64
	fv := reflect.ValueOf(&n).Elem()
	if err := setFieldValue(fv, reflect.Int64, []string{"not-a-number"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetFieldValue_Uint(t *testing.T) {
	var n uint64
	fv := reflect.ValueOf(&n).Elem()
	if err := setFieldValue(fv, reflect.Uint64, []string{"99"}); err != nil {
		t.Fatal(err)
	}
	if n != 99 {
		t.Fatalf("got %d, want %d", n, 99)
	}
}

func TestSetFieldValue_Uint_Error(t *testing.T) {
	var n uint64
	fv := reflect.ValueOf(&n).Elem()
	if err := setFieldValue(fv, reflect.Uint64, []string{"-1"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetFieldValue_Float(t *testing.T) {
	var f float64
	fv := reflect.ValueOf(&f).Elem()
	if err := setFieldValue(fv, reflect.Float64, []string{"3.14"}); err != nil {
		t.Fatal(err)
	}
	if f != 3.14 {
		t.Fatalf("got %f, want %f", f, 3.14)
	}
}

func TestSetFieldValue_Bool_True(t *testing.T) {
	for _, val := range []string{"true", "TRUE", "True", "1", "yes", "YES", "Yes", "on", "ON", "On"} {
		var b bool
		fv := reflect.ValueOf(&b).Elem()
		if err := setFieldValue(fv, reflect.Bool, []string{val}); err != nil {
			t.Fatalf("setFieldValue(bool, %q): %v", val, err)
		}
		if !b {
			t.Fatalf("setFieldValue(bool, %q) = false, want true", val)
		}
	}
}

func TestSetFieldValue_Bool_False(t *testing.T) {
	for _, val := range []string{"false", "FALSE", "False", "0", "no", "random"} {
		var b bool
		fv := reflect.ValueOf(&b).Elem()
		if err := setFieldValue(fv, reflect.Bool, []string{val}); err != nil {
			t.Fatalf("setFieldValue(bool, %q): %v", val, err)
		}
		if b {
			t.Fatalf("setFieldValue(bool, %q) = true, want false", val)
		}
	}
}

func TestSetFieldValue_Slice(t *testing.T) {
	var vals []string
	fv := reflect.ValueOf(&vals).Elem()
	if err := setFieldValue(fv, reflect.Slice, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	if len(vals) != 3 || vals[0] != "a" || vals[1] != "b" || vals[2] != "c" {
		t.Fatalf("got %v, want [a b c]", vals)
	}
}

func TestSetFieldValue_Default(t *testing.T) {
	var m map[string]string
	fv := reflect.ValueOf(&m).Elem()
	if err := setFieldValue(fv, reflect.Map, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("should not modify map type")
	}
}
