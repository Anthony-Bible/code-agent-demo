// Package logger provides a centralized, modular logging infrastructure.
//
// It defines a concrete implementation of the port.Logger interface backed by
// the standard library slog package, along with factory functions for creating
// loggers configured from application settings.
//
// Switching the underlying logging library only requires updating this package.
// All consumers depend on the port.Logger interface, so no other code needs to
// change when the implementation is swapped.
//
// Usage:
//
//	// In the DI container:
//	log := logger.New(logger.Options{Level: "info", Format: "text"})
//
//	// In tests:
//	log := logger.NewNop()
//
//	// Attach contextual fields:
//	log = log.With("investigation_id", id)
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// Options configures the logger produced by New.
// Zero values are treated as sensible defaults (info level, text format).
type Options struct {
	// Level sets the minimum log level. Accepted values (case-insensitive):
	// "debug", "info", "warn", "error". Defaults to "info".
	Level string
	// Format selects the output format. Accepted values: "json", "text".
	// Defaults to "text".
	Format string
}

// slogLogger wraps *slog.Logger to satisfy the port.Logger interface.
type slogLogger struct {
	l *slog.Logger
}

func (s *slogLogger) Debug(msg string, args ...any) { s.l.Debug(msg, args...) }
func (s *slogLogger) Info(msg string, args ...any)  { s.l.Info(msg, args...) }
func (s *slogLogger) Warn(msg string, args ...any)  { s.l.Warn(msg, args...) }
func (s *slogLogger) Error(msg string, args ...any) { s.l.Error(msg, args...) }
func (s *slogLogger) Log(level string, msg string, args ...any) {
	switch level {
	case "debug":
		s.l.Debug(msg, args...)
	case "info":
		s.l.Info(msg, args...)
	case "warn", "warning":
		s.l.Warn(msg, args...)
	case "error":
		s.l.Error(msg, args...)
	default:
		s.l.Info(msg, args...)
	}
}

// With returns a new Logger that includes the given key-value pairs in every
// subsequent log record.
func (s *slogLogger) With(args ...any) port.Logger {
	return &slogLogger{l: s.l.With(args...)}
}

// New creates a port.Logger configured by the given Options.
// This is the recommended constructor for production use; it reads the level
// and format fields so the caller can control logging behaviour centrally
// without the logger package depending on any application-specific config type.
func New(opts Options) port.Logger {
	level := parseLevel(opts.Level)
	handlerOpts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.EqualFold(opts.Format, "json") {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}
	return &slogLogger{l: slog.New(handler)}
}

// NewDefault creates a port.Logger with text format at info level, writing to stderr.
func NewDefault() port.Logger {
	return New(Options{})
}

// NewNop creates a port.Logger that silently discards all log messages.
// Use this in tests where log output is irrelevant.
func NewNop() port.Logger {
	return &slogLogger{l: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// FromSlog wraps an existing *slog.Logger as a port.Logger.
// Use this to bridge code that already constructs a *slog.Logger directly.
func FromSlog(l *slog.Logger) port.Logger {
	return &slogLogger{l: l}
}

// parseLevel converts a string level name to slog.Level.
// Accepted values are case-insensitive: "debug", "info", "warn" (or "warning"), "error".
// Any other value, including an empty string, defaults to slog.LevelInfo.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// Empty string and unrecognised values fall back to info.
		return slog.LevelInfo
	}
}
