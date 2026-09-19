package bytesconv

import (
	"testing"
	"unicode/utf8"
)

func TestStringToBytes_Empty(t *testing.T) {
	b := StringToBytes("")
	if len(b) != 0 {
		t.Fatal("empty string should return an empty slice")
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

func TestStringToBytes_ImmutableObservation(t *testing.T) {
	s := "fixed"
	b := StringToBytes(s)
	if len(b) != 5 || string(b) != "fixed" {
		t.Fatal("unexpected content")
	}
}
