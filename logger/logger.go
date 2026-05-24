package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"
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

// Logger provides structured logging with slog backend.
type Logger struct {
	logger  *slog.Logger
	level   Level
	out     io.Writer
	leveler *slog.LevelVar
}

// New creates a new logger with the specified level and output writer.
// The level can be changed dynamically via SetLevel.
func New(level Level, out io.Writer) *Logger {
	leveler := &slog.LevelVar{}
	leveler.Set(slog.Level(level))

	// Intercept and cleanly normalize custom TRACE/FATAL strings inside the output writer
	replaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.LevelKey {
			l := a.Value.Any().(slog.Level)
			switch l {
			case slog.Level(TRACE):
				a.Value = slog.StringValue("TRACE")
			case slog.Level(FATAL):
				a.Value = slog.StringValue("FATAL")
			}
		}
		return a
	}

	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		Level:       leveler,
		ReplaceAttr: replaceAttr,
		AddSource:   true, // Automatically captures correct file:line details
	})

	return &Logger{
		level:   level,
		out:     out,
		leveler: leveler,
		logger:  slog.New(handler),
	}
}

// Default creates a logger with INFO level and stderr output.
func Default() *Logger {
	return New(INFO, os.Stderr)
}

// SetLevel changes the minimum log level dynamically.
func (l *Logger) SetLevel(level Level) {
	l.level = level
	l.leveler.Set(slog.Level(level))
}

// Enabled checks if the given level is permitted to print.
func (l *Logger) Enabled(level Level) bool {
	return slog.Level(level) >= l.leveler.Level()
}

// log writes a log entry if the level is enabled, adjusting call depth natively
// to track correct file lines instead of the wrapper helper line.
func (l *Logger) log(level Level, message string, args ...any) {
	slogLevel := slog.Level(level)
	if !l.logger.Enabled(context.Background(), slogLevel) {
		return
	}

	// Capture caller PC to fix source file/line reporting accuracy
	var pc uintptr
	var pcs [1]uintptr
	// Skip 3 frames: runtime.Callers -> l.log -> Public Wrapper (e.g. Info) -> Real Application Caller
	if n := runtime.Callers(3, pcs[:]); n > 0 {
		pc = pcs[0]
	}

	r := slog.NewRecord(time.Now(), slogLevel, message, pc)
	r.Add(args...)
	_ = l.logger.Handler().Handle(context.Background(), r)

	if level >= FATAL {
		os.Exit(1)
	}
}

// Trace logs a trace message with key-value fields.
func (l *Logger) Trace(message string, args ...any) { l.log(TRACE, message, args...) }

// Debug logs a debug message with key-value fields.
func (l *Logger) Debug(message string, args ...any) { l.log(DEBUG, message, args...) }

// Info logs an info message with key-value fields.
func (l *Logger) Info(message string, args ...any) { l.log(INFO, message, args...) }

// Warn logs a warning message with key-value fields.
func (l *Logger) Warn(message string, args ...any) { l.log(WARN, message, args...) }

// Error logs an error message with key-value fields.
func (l *Logger) Error(message string, args ...any) { l.log(ERROR, message, args...) }

// Fatal logs a fatal message, flushes, and invokes os.Exit(1).
func (l *Logger) Fatal(message string, args ...any) { l.log(FATAL, message, args...) }

// --- Global Facade Layer ---

var defaultLogger = Default()

func SetDefault(l *Logger)          { defaultLogger = l }
func SetLevel(level Level)          { defaultLogger.SetLevel(level) }
func Trace(msg string, args ...any) { defaultLogger.log(TRACE, msg, args...) }
func Debug(msg string, args ...any) { defaultLogger.log(DEBUG, msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.log(INFO, msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.log(WARN, msg, args...) }
func Error(msg string, args ...any) { defaultLogger.log(ERROR, msg, args...) }
func Fatal(msg string, args ...any) { defaultLogger.log(FATAL, msg, args...) }
