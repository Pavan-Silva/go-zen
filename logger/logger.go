package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// Level represents the severity of log entries
type Level int

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

// Logger provides structured logging with optional color support and plain framework-style formatting.
type Logger struct {
	level    Level
	logger   *log.Logger
}

// New creates a new logger with the specified level, output writer, and optional color support.
func New(level Level, out io.Writer) *Logger {
	return &Logger{
		level:    level,
		logger:   log.New(out, "", 0), // We'll handle formatting ourselves
	}
}

// Default creates a logger with INFO level and stderr output. Colors are disabled by default.
func Default() *Logger {
	return New(INFO, os.Stderr)
}

// SetLevel changes the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.level = level
}

// log writes a log entry if the level is enabled
func (l *Logger) log(level Level, message string, args ...any) {
	if level < l.level {
		return
	}

	// Format the message
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelStr := level.String()

	// Pad level to 5 characters for alignment
	if len(levelStr) < 5 {
		levelStr += strings.Repeat(" ", 5-len(levelStr))
	}

	prefix := fmt.Sprintf("[%s]", levelStr)
	formatted := fmt.Sprintf("%s %s %s", timestamp, prefix, fmt.Sprintf(message, args...))

	l.logger.Println(formatted)

	if level == FATAL {
		os.Exit(1)
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