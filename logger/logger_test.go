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

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
	if !strings.Contains(lines[0], "TRACE") {
		t.Fatalf("line 0 missing TRACE: %s", lines[0])
	}
	if !strings.Contains(lines[4], "ERROR") {
		t.Fatalf("line 4 missing ERROR: %s", lines[4])
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
		{FATAL, "FATAL"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLevelPaddedString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{INFO, "INFO "},
		{WARN, "WARN "},
		{ERROR, "ERROR"},
	}

	for _, tt := range tests {
		got := tt.level.paddedString()
		if got != tt.want {
			t.Errorf("Level(%d).paddedString() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestSetLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(INFO, buf)

	l.Debug("before")
	if buf.Len() != 0 {
		t.Fatal("debug should not be logged at INFO level")
	}

	l.SetLevel(TRACE)
	l.Debug("after")
	if buf.Len() == 0 {
		t.Fatal("debug should be logged at TRACE level")
	}
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
	var exitCode int
	buf := &bytes.Buffer{}
	l := New(FATAL, buf)
	l.exitFn = func(code int) {
		exitCode = code
	}

	l.Fatal("fatal msg")

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(buf.String(), "fatal msg") {
		t.Fatalf("fatal message not logged: %s", buf.String())
	}
}

func TestFmtArguments(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(INFO, buf)

	l.Info("user %s logged in from %s", "john", "127.0.0.1")

	if !strings.Contains(buf.String(), "user john logged in from 127.0.0.1") {
		t.Fatalf("format not applied: %s", buf.String())
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
