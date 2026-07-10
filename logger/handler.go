package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// bufPool pools byte buffers for log line formatting.
var bufPool = sync.Pool{
	New: func() any { return make([]byte, 0, 256) },
}

// ANSI color codes for level-based log coloring.
var (
	resetColor = []byte("\033[0m")
	redBold    = []byte("\033[1;31m")
	red        = []byte("\033[31m")
	yellow     = []byte("\033[33m")
	green      = []byte("\033[32m")
	gray       = []byte("\033[90m")
)

// levelANSI returns the ANSI color code for the given log level.
func levelANSI(level slog.Level) []byte {
	switch {
	case level >= slog.Level(FATAL):
		return redBold
	case level >= slog.LevelError:
		return red
	case level >= slog.LevelWarn:
		return yellow
	case level >= slog.LevelInfo:
		return green
	default:
		return gray
	}
}

// levelLabel returns the string label for the given log level.
func levelLabel(level slog.Level) string {
	switch level {
	case slog.Level(TRACE):
		return "TRACE"
	case slog.Level(DEBUG):
		return "DEBUG"
	case slog.Level(INFO):
		return "INFO"
	case slog.Level(WARN):
		return "WARN"
	case slog.Level(ERROR):
		return "ERROR"
	case slog.Level(FATAL):
		return "FATAL"
	default:
		return level.String()
	}
}

// useColor reports whether the writer supports ANSI color output.
// Honors the NO_COLOR environment variable (https://no-color.org).
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// consoleHandler is a slog.Handler that writes colored, human-readable log
// lines to an io.Writer. Output format:
//
//	YYYY-MM-DD HH:MM:SS  LEVEL  file:line  message  key=val
type consoleHandler struct {
	out       io.Writer
	level     slog.Leveler
	addSource bool
	noColor   bool
	attrs     []slog.Attr
	group     string
}

// newConsoleHandler creates a colored console handler that writes to out.
func newConsoleHandler(out io.Writer, level slog.Leveler, addSource bool) *consoleHandler {
	return &consoleHandler{
		out:       out,
		level:     level,
		addSource: addSource,
		noColor:   !useColor(out),
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle formats and writes a log record to the output writer.
func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	buf := bufPool.Get().([]byte)
	defer func() {
		bufPool.Put(buf[:0])
	}()

	buf = r.Time.AppendFormat(buf, time.DateTime)
	buf = append(buf, "  "...)

	if h.noColor {
		buf = append(buf, levelLabel(r.Level)...)
	} else {
		buf = append(buf, levelANSI(r.Level)...)
		buf = append(buf, levelLabel(r.Level)...)
		buf = append(buf, resetColor...)
	}
	buf = append(buf, "  "...)

	if h.addSource && r.PC != 0 {
		frames := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := frames.Next()
		buf = append(buf, f.File...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(f.Line), 10)
		buf = append(buf, "  "...)
	}

	buf = append(buf, r.Message...)

	for _, a := range h.attrs {
		buf = append(buf, "  "...)
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
	}

	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, "  "...)
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
		return true
	})

	buf = append(buf, '\n')

	_, err := h.out.Write(buf)
	return err
}

// WithAttrs returns a new handler with the given attributes attached.
func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.attrs = append(h2.attrs, attrs...)
	return &h2
}

// WithGroup returns a new handler with the given group name prefixed.
func (h *consoleHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	if h2.group != "" {
		h2.group = h2.group + "." + name
	} else {
		h2.group = name
	}
	return &h2
}
