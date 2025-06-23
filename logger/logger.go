package logger

import (
	"context"
)

type contextKey int

const (
	contextLoggerKey contextKey = iota + 1
)

// Logger interface for external implementation to enable logging for outbox.
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Error(args ...interface{})

	WithError(err error) Logger
	WithContext(ctx context.Context) Logger
	WithField(key string, value interface{}) Logger
}

// FromContext extracts logger from context or default logger if not exists.
func FromContext(ctx context.Context) Logger {
	log, ok := ctx.Value(contextLoggerKey).(Logger)
	if !ok {
		return NewNoop()
	}

	return log
}

// WithLogger set specific logger to context value.
func WithLogger(ctx context.Context, log Logger) context.Context {
	return context.WithValue(ctx, contextLoggerKey, log)
}
