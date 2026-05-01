package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Level represents the severity of log entries
type Level int32

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// paddedString returns the level string padded to 5 characters for alignment.
// Padding is static to avoid per-call allocations.
func (l Level) paddedString() string {
	switch l {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO "
	case WARN:
		return "WARN "
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNO"
	}
}

// Logger provides structured logging with plain framework-style formatting.
type Logger struct {
	level  atomic.Int32
	out    io.Writer
	mu     sync.Mutex // protects writes to out
	exitFn func(int)  // injectable for testing; defaults to os.Exit
}

// New creates a new logger with the specified level and output writer.
func New(level Level, out io.Writer) *Logger {
	l := &Logger{
		out:    out,
		exitFn: os.Exit,
	}
	l.level.Store(int32(level))
	return l
}

// Default creates a logger with INFO level and stderr output.
func Default() *Logger {
	return New(INFO, os.Stderr)
}

// SetLevel changes the minimum log level atomically.
func (l *Logger) SetLevel(level Level) {
	l.level.Store(int32(level))
}

// log writes a log entry if the level is enabled
func (l *Logger) log(level Level, message string, args ...any) {
	if Level(l.level.Load()) > level {
		return
	}

	var msg string
	if len(args) == 0 {
		msg = message
	} else {
		msg = fmt.Sprintf(message, args...)
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("%s [%s] %s\n", timestamp, level.paddedString(), msg)

	l.mu.Lock()
	//nolint:errcheck // matching stdlib log behaviour of ignoring write errors
	_, _ = io.WriteString(l.out, line)
	l.mu.Unlock()

	if level == FATAL {
		l.exitFn(1)
	}
}

// Trace logs a trace message
func (l *Logger) Trace(message string, args ...any) {
	l.log(TRACE, message, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(message string, args ...any) {
	l.log(DEBUG, message, args...)
}

// Info logs an info message
func (l *Logger) Info(message string, args ...any) {
	l.log(INFO, message, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, args ...any) {
	l.log(WARN, message, args...)
}

// Error logs an error message
func (l *Logger) Error(message string, args ...any) {
	l.log(ERROR, message, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, args ...any) {
	l.log(FATAL, message, args...)
}

// Global logger instance
var defaultLogger = Default()

// SetDefault replaces the global logger instance.
// Useful for testing or redirecting logs to a custom output.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// SetLevel sets the global logger level
func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// Trace logs to the global logger
func Trace(message string, args ...any) {
	defaultLogger.Trace(message, args...)
}

// Debug logs to the global logger
func Debug(message string, args ...any) {
	defaultLogger.Debug(message, args...)
}

// Info logs to the global logger
func Info(message string, args ...any) {
	defaultLogger.Info(message, args...)
}

// Warn logs to the global logger
func Warn(message string, args ...any) {
	defaultLogger.Warn(message, args...)
}

// Error logs to the global logger
func Error(message string, args ...any) {
	defaultLogger.Error(message, args...)
}

// Fatal logs to the global logger
func Fatal(message string, args ...any) {
	defaultLogger.Fatal(message, args...)
}
