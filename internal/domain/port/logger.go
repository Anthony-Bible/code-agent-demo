// Package port defines the interface contracts for the domain layer.
// This file defines the Logger interface, providing a structured logging
// abstraction that all layers can use without depending on a specific
// logging implementation.
package port

// Logger is a structured logger interface that provides leveled logging with
// key-value pair arguments.
//
// Implementations are provided by the infrastructure/logger package, which
// wraps the standard library slog.Logger. Switching logging libraries only
// requires changing the infrastructure implementation without modifying any
// business logic.
type Logger interface {
	// Debug logs a message at debug level with optional key-value pairs.
	Debug(msg string, args ...any)
	// Info logs a message at info level with optional key-value pairs.
	Info(msg string, args ...any)
	// Warn logs a message at warn level with optional key-value pairs.
	Warn(msg string, args ...any)
	// Error logs a message at error level with optional key-value pairs.
	Error(msg string, args ...any)
	// Log logs a message at the specified level with optional key-value pairs.
	// The level string is interpreted by the underlying implementation (e.g., "debug", "info", "warn", "error").
	Log(level string, msg string, args ...any)
	// With returns a new Logger with the given key-value pairs added to every
	// subsequent log message. Use this to attach contextual fields such as
	// investigation_id or session_id.
	With(args ...any) Logger
}

// NopLogger is a no-op Logger implementation that silently discards all messages.
// Use it as a safe default when a real logger has not been provided, for example
// during testing or in legacy call sites that have not been migrated to inject a
// logger yet.
type NopLogger struct{}

func (NopLogger) Debug(_ string, _ ...any)         {}
func (NopLogger) Info(_ string, _ ...any)          {}
func (NopLogger) Warn(_ string, _ ...any)          {}
func (NopLogger) Error(_ string, _ ...any)         {}
func (NopLogger) Log(_ string, _ string, _ ...any) {}

var nopLoggerInstance = &NopLogger{}

// With returns the same NopLogger instance since it has no state.
// This returns a singleton to avoid unnecessary allocations on each call.
func (NopLogger) With(_ ...any) Logger { return nopLoggerInstance }
