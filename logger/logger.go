package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Level represents the severity of log entries.
type Level int32

const (
	TRACE Level = -4 // Custom trace level below Debug
	DEBUG Level = 0  // slog.LevelDebug = 0
	INFO  Level = 4  // slog.LevelInfo = 4
	WARN  Level = 8  // slog.LevelWarn = 8
	ERROR Level = 12 // slog.LevelError = 12
	FATAL Level = 16 // Custom fatal level above Error
)

// String returns the string representation of the level.
func (l Level) String() string {
	switch {
	case l <= TRACE:
		return "TRACE"
	case l <= DEBUG:
		return "DEBUG"
	case l <= INFO:
		return "INFO"
	case l <= WARN:
		return "WARN"
	case l <= ERROR:
		return "ERROR"
	default:
		return "FATAL"
	}
}

// toSlogLevel converts our Level to slog.Level.
func toSlogLevel(l Level) slog.Level {
	return slog.Level(l)
}

// Logger provides structured logging with slog backend.
type Logger struct {
	logger *slog.Logger
	level  Level
	out    io.Writer
}

// New creates a new logger with the specified level and output writer.
func New(level Level, out io.Writer) *Logger {
	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: toSlogLevel(level),
	})
	return &Logger{
		logger: slog.New(handler),
		level:  level,
		out:    out,
	}
}

// Default creates a logger with INFO level and stderr output.
func Default() *Logger {
	return New(INFO, os.Stderr)
}

// SetLevel changes the minimum log level.
// Note: slog doesn't support dynamic level changes, so this is a no-op.
// Set the level at creation time via New().
func (l *Logger) SetLevel(level Level) {
	l.level = level
}

// enabled checks if the level is enabled.
func (l *Logger) enabled(level Level) bool {
	return level >= l.level
}

// log writes a log entry if the level is enabled.
func (l *Logger) log(level Level, message string, args ...any) {
	if !l.enabled(level) {
		return
	}

	if len(args) > 0 {
		l.logger.Log(context.TODO(), toSlogLevel(level), fmt.Sprintf(message, args...))
	} else {
		l.logger.Log(context.TODO(), toSlogLevel(level), message)
	}

	if level >= FATAL {
		os.Exit(1)
	}
}

// Trace logs a trace message.
func (l *Logger) Trace(message string, args ...any) {
	l.log(TRACE, message, args...)
}

// Debug logs a debug message.
func (l *Logger) Debug(message string, args ...any) {
	l.log(DEBUG, message, args...)
}

// Info logs an info message.
func (l *Logger) Info(message string, args ...any) {
	l.log(INFO, message, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(message string, args ...any) {
	l.log(WARN, message, args...)
}

// Error logs an error message.
func (l *Logger) Error(message string, args ...any) {
	l.log(ERROR, message, args...)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(message string, args ...any) {
	l.log(FATAL, message, args...)
}

// Global logger instance.
var defaultLogger = Default()

// SetDefault replaces the global logger instance.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// SetLevel sets the global logger level.
func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// Trace logs to the global logger.
func Trace(message string, args ...any) {
	defaultLogger.Trace(message, args...)
}

// Debug logs to the global logger.
func Debug(message string, args ...any) {
	defaultLogger.Debug(message, args...)
}

// Info logs to the global logger.
func Info(message string, args ...any) {
	defaultLogger.Info(message, args...)
}

// Warn logs to the global logger.
func Warn(message string, args ...any) {
	defaultLogger.Warn(message, args...)
}

// Error logs to the global logger.
func Error(message string, args ...any) {
	defaultLogger.Error(message, args...)
}

// Fatal logs to the global logger.
func Fatal(message string, args ...any) {
	defaultLogger.Fatal(message, args...)
}
