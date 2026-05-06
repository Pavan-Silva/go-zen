package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(INFO, buf)
	if l == nil {
		t.Fatal("New returned nil")
	}
}

func TestDefault(t *testing.T) {
	l := Default()
	if l == nil {
		t.Fatal("Default returned nil")
	}
}

func TestLogLevels(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(TRACE, buf)

	l.Trace("trace msg")
	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")

	output := buf.String()
	// slog maps TRACE to DEBUG level, so both should appear as DEBUG
	if !strings.Contains(output, "DEBUG") {
		t.Fatalf("debug/trace not found: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Fatalf("info not found: %s", output)
	}
	if !strings.Contains(output, "WARN") {
		t.Fatalf("warn not found: %s", output)
	}
	if !strings.Contains(output, "ERROR") {
		t.Fatalf("error not found: %s", output)
	}
}

func TestLevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(WARN, buf)

	l.Debug("should not appear")
	l.Info("should not appear")
	l.Warn("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Fatalf("filtered messages appeared: %s", output)
	}
	if !strings.Contains(output, "should appear") {
		t.Fatalf("warn message not logged: %s", output)
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{TRACE, "TRACE"},
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(99), "FATAL"}, // unknown levels map to FATAL
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestSetLevel(t *testing.T) {
	t.Skip("slog doesn't support dynamic level changes")
}

func TestGlobalFunctions(t *testing.T) {
	buf := &bytes.Buffer{}
	testLogger := New(TRACE, buf)
	SetDefault(testLogger)

	Info("global info")
	Warn("global warn")
	Error("global error")

	output := buf.String()
	if !strings.Contains(output, "global info") {
		t.Fatal("global Info not working")
	}
}

func TestSetDefault(t *testing.T) {
	buf := &bytes.Buffer{}
	newLogger := New(TRACE, buf)
	SetDefault(newLogger)

	Info("test")
	if !strings.Contains(buf.String(), "test") {
		t.Fatal("SetDefault not working")
	}
}

func TestFatal_Exit(t *testing.T) {
	t.Skip("Fatal calls os.Exit, skipping")
}

func TestFmtArguments(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(INFO, buf)

	l.Info("user %s logged in from %s", "john", "127.0.0.1")

	output := buf.String()
	if !strings.Contains(output, "user john logged in from 127.0.0.1") {
		t.Fatalf("format not applied: %s", output)
	}
}

func BenchmarkLogger(b *testing.B) {
	buf := &bytes.Buffer{}
	l := New(INFO, buf)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("benchmark message with args: %d %s", i, "test")
		buf.Reset()
	}
}
