// Package logging builds the diagnostic logger used by vault-manager.
//
// Diagnostic logs are separate from regular command output: they go to
// stderr by default and are filtered by level, while results meant for
// further processing stay on stdout.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// LevelTrace is more verbose than slog.LevelDebug and enables HTTP tracing.
const LevelTrace = slog.Level(-8)

const (
	DefaultLevel  = "info"
	DefaultFormat = "text"
)

// Stable attribute keys shared across the code base.
const (
	KeyCommand  = "command"
	KeyPath     = "path"
	KeyCount    = "count"
	KeyDuration = "duration"
	KeyError    = "error"
)

// Config describes how a logger should be built. Empty fields fall back to
// DefaultLevel, DefaultFormat and os.Stderr.
type Config struct {
	Level  string
	Format string
	Writer io.Writer
}

// ParseLevel converts one of error, warn, info, debug or trace to a slog.Level.
func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error":
		return slog.LevelError, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "trace":
		return LevelTrace, nil
	}
	return 0, fmt.Errorf("unknown log level '%s' (expected one of: error, warn, info, debug, trace)", value)
}

// ParseFormat validates a log format and returns its canonical name.
func ParseFormat(value string) (string, error) {
	switch f := strings.ToLower(strings.TrimSpace(value)); f {
	case "":
		return DefaultFormat, nil
	case "text", "json":
		return f, nil
	}
	return "", fmt.Errorf("unknown log format '%s' (expected one of: text, json)", value)
}

// New builds a logger from cfg. It never touches slog's global default logger.
func New(cfg Config) (*slog.Logger, error) {
	level, err := ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	format, err := ParseFormat(cfg.Format)
	if err != nil {
		return nil, err
	}

	w := cfg.Writer
	if w == nil {
		w = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceLevelName,
	}

	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h), nil
}

// Discard returns a logger that drops every record.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// OrDiscard returns l, or a discarding logger when l is nil.
func OrDiscard(l *slog.Logger) *slog.Logger {
	if l == nil {
		return Discard()
	}
	return l
}

// replaceLevelName renders LevelTrace as "TRACE" instead of "DEBUG-4".
func replaceLevelName(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey && len(groups) == 0 {
		if lvl, ok := a.Value.Any().(slog.Level); ok && lvl <= LevelTrace {
			a.Value = slog.StringValue("TRACE")
		}
	}
	return a
}

// LegacyDebugEnabled reports whether the deprecated DEBUG environment
// variable value turns debugging on.
func LegacyDebugEnabled(value string) bool {
	d := strings.ToLower(strings.TrimSpace(value))
	return d != "" && d != "false" && d != "0" && d != "no" && d != "off"
}

// Setup builds the application logger. level and format are the values
// already merged from command-line flags and VAULT_MANAGER_LOG_* variables
// (flags win). When no level was given and the deprecated DEBUG variable is
// enabled, the level falls back to trace and a deprecation warning is logged.
func Setup(level, format, legacyDebug string, w io.Writer) (*slog.Logger, error) {
	useLegacy := strings.TrimSpace(level) == "" && LegacyDebugEnabled(legacyDebug)
	if useLegacy {
		level = "trace"
	}

	l, err := New(Config{Level: level, Format: format, Writer: w})
	if err != nil {
		return nil, err
	}
	if useLegacy {
		l.Warn("DEBUG environment variable is deprecated; use --log-level=trace or VAULT_MANAGER_LOG_LEVEL=trace instead")
	}
	return l, nil
}
