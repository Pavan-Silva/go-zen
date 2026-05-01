package zen

import (
	"strings"
	"testing"
)

func TestEncodeSSEData_String(t *testing.T) {
	result, err := encodeSSEData("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Fatalf("result = %q, want %q", result, "hello world")
	}
}

func TestEncodeSSEData_StringWithNewlines(t *testing.T) {
	result, err := encodeSSEData("line1\nline2")
	if err != nil {
		t.Fatal(err)
	}
	if result != "line1\ndata: line2" {
		t.Fatalf("result = %q, want %q", result, "line1\ndata: line2")
	}
}

func TestEncodeSSEData_Bytes(t *testing.T) {
	result, err := encodeSSEData([]byte("binary data"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "binary data" {
		t.Fatalf("result = %q", result)
	}
}

func TestEncodeSSEData_BytesWithNewlines(t *testing.T) {
	result, err := encodeSSEData([]byte("a\nb"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "a\ndata: b" {
		t.Fatalf("result = %q", result)
	}
}

func TestEncodeSSEData_JSON(t *testing.T) {
	type msg struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	}
	result, err := encodeSSEData(msg{ID: 1, Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"id":1`) && !strings.Contains(result, `"id": 1`) {
		t.Fatalf("result should contain id field: %q", result)
	}
}

func TestEncodeSSEData_CRLFNormalization(t *testing.T) {
	result, err := encodeSSEData("line1\r\nline2\rline3")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "\r") {
		t.Fatalf("result should not contain \\r: %q", result)
	}
}

func TestHasSSEContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"text/event-stream", true},
		{"text/event-stream; charset=utf-8", true},
		{"TEXT/EVENT-STREAM", true},
		{"text/event-stream ", true},
		{"application/json", false},
		{"", false},
	}

	for _, tt := range tests {
		got := hasSSEContentType(tt.input)
		if got != tt.want {
			t.Errorf("hasSSEContentType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
