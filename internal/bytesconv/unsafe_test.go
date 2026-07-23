package bytesconv

import (
	"testing"
	"unicode/utf8"
)

func TestStringToBytes_Empty(t *testing.T) {
	b := StringToBytes("")
	if b != nil {
		t.Fatal("empty string should return nil")
	}
}

func TestStringToBytes_ASCII(t *testing.T) {
	s := "hello"
	b := StringToBytes(s)
	if len(b) != len(s) {
		t.Fatalf("len = %d, want %d", len(b), len(s))
	}
	if string(b) != s {
		t.Fatal("content mismatch")
	}
}

func TestStringToBytes_Unicode(t *testing.T) {
	s := "日本語"
	b := StringToBytes(s)
	if len(b) != len(s) {
		t.Fatalf("len = %d, want %d", len(b), len(s))
	}
	if !utf8.Valid(b) {
		t.Fatal("bytes should be valid UTF-8")
	}
	if string(b) != s {
		t.Fatal("content mismatch")
	}
}

func TestStringToBytes_SameData(t *testing.T) {
	s := "no allocation"
	b1 := StringToBytes(s)
	b2 := StringToBytes(s)
	if len(b1) == 0 || len(b2) == 0 {
		t.Fatal("expected non-empty slices")
	}
	if &b1[0] != &b2[0] {
		t.Fatal("must point to same underlying string data")
	}
}

func TestBytesToString_Empty(t *testing.T) {
	s := BytesToString(nil)
	if s != "" {
		t.Fatal("nil slice should return empty string")
	}

	s = BytesToString([]byte{})
	if s != "" {
		t.Fatal("empty slice should return empty string")
	}
}

func TestBytesToString_ASCII(t *testing.T) {
	b := []byte("hello")
	s := BytesToString(b)
	if s != "hello" {
		t.Fatal("content mismatch")
	}
}

func TestBytesToString_Unicode(t *testing.T) {
	b := []byte("日本語")
	s := BytesToString(b)
	if s != "日本語" {
		t.Fatal("content mismatch")
	}
}

func TestBytesToString_SameData(t *testing.T) {
	b := []byte("no allocation")
	s1 := BytesToString(b)
	s2 := BytesToString(b)
	if s1 != s2 {
		t.Fatal("must produce same string")
	}
	if len(s1) == 0 {
		t.Fatal("expected non-empty string")
	}
}

func TestBytesToString_RoundTrip(t *testing.T) {
	original := "hello world 🌍"
	b := StringToBytes(original)
	s := BytesToString(b)
	if s != original {
		t.Fatal("round trip failed")
	}
}

func TestStringToBytes_ImmutableObservation(t *testing.T) {
	s := "fixed"
	b := StringToBytes(s)
	if len(b) != 5 || string(b) != "fixed" {
		t.Fatal("unexpected content")
	}
}

func TestBytesToString_NilInput(t *testing.T) {
	var b []byte
	s := BytesToString(b)
	if s != "" {
		t.Fatal("nil input should produce empty string")
	}
}
