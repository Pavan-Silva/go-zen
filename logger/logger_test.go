package logger

import (
	"bytes"
	"os"
	"os/exec"
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
	if !strings.Contains(output, "TRACE") {
		t.Fatalf("trace not found: %s", output)
	}
	if !strings.Contains(output, "WARN") {
		t.Fatalf("info/warn not found: %s", output)
	}
	if !strings.Contains(output, "ERROR") {
		t.Fatalf("warn/error not found: %s", output)
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
	buf := &bytes.Buffer{}
	l := New(INFO, buf)

	l.Info("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Fatal("info message should be visible")
	}

	buf.Reset()
	l.Debug("hidden")
	if strings.Contains(buf.String(), "hidden") {
		t.Fatal("debug should be hidden at INFO level")
	}

	l.SetLevel(DEBUG)
	buf.Reset()
	l.Debug("now visible")
	if !strings.Contains(buf.String(), "now visible") {
		t.Fatal("debug should be visible after SetLevel(DEBUG)")
	}
}

func TestEnabled(t *testing.T) {
	l := New(INFO, &bytes.Buffer{})

	if l.Enabled(TRACE) {
		t.Fatal("TRACE should not be enabled at INFO level")
	}
	if l.Enabled(DEBUG) {
		t.Fatal("DEBUG should not be enabled at INFO level")
	}
	if !l.Enabled(INFO) {
		t.Fatal("INFO should be enabled at INFO level")
	}
	if !l.Enabled(WARN) {
		t.Fatal("WARN should be enabled at INFO level")
	}
	if !l.Enabled(ERROR) {
		t.Fatal("ERROR should be enabled at INFO level")
	}
	if !l.Enabled(FATAL) {
		t.Fatal("FATAL should be enabled at INFO level")
	}

	l.SetLevel(TRACE)
	if !l.Enabled(TRACE) {
		t.Fatal("TRACE should be enabled at TRACE level")
	}
}

func TestEnabled_DefaultLogger(t *testing.T) {
	if !Default().Enabled(INFO) {
		t.Fatal("default logger should have INFO enabled")
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

func TestGlobalTraceDebug(t *testing.T) {
	buf := &bytes.Buffer{}
	testLogger := New(TRACE, buf)
	SetDefault(testLogger)

	Trace("global trace")
	Debug("global debug")

	output := buf.String()
	if !strings.Contains(output, "global trace") {
		t.Fatal("global Trace not working")
	}
	if !strings.Contains(output, "global debug") {
		t.Fatal("global Debug not working")
	}
}

func TestSetLevel_Global(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(INFO, buf)
	SetDefault(l)

	SetLevel(TRACE)
	if !l.Enabled(TRACE) {
		t.Fatal("global SetLevel should enable TRACE")
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

func TestFatal_LogsBeforeExit(t *testing.T) {
	if os.Getenv("TEST_FATAL_EXIT") == "1" {
		l := New(FATAL, os.Stdout)
		l.Fatal("fatal message", "key", "val")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), "TEST_FATAL_EXIT=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Fatal should exit with code 1")
	}
	if !strings.Contains(string(out), "fatal message") {
		t.Fatal("Fatal should log message before exit")
	}
	if !strings.Contains(string(out), "FATAL") {
		t.Fatal("FATAL level should appear in output")
	}
	if !strings.Contains(string(out), "key") || !strings.Contains(string(out), "val") {
		t.Fatal("Fatal should include args")
	}
}

func TestFmtArguments(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(INFO, buf)

	l.Info("user logged in", "user", "john", "ip", "127.0.0.1")

	output := buf.String()
	if !strings.Contains(output, "user logged in") {
		t.Fatalf("message not found: %s", output)
	}
	if !strings.Contains(output, "john") || !strings.Contains(output, "127.0.0.1") {
		t.Fatalf("args not found: %s", output)
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
