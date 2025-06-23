package logger

import (
	"context"
)

// Noop is no operations imlementation of Logger interface.
type Noop struct {
}

func NewNoop() *Noop {
	return &Noop{}
}

func (nw *Noop) Debug(args ...interface{}) {
}

func (nw *Noop) Info(args ...interface{}) {
}

func (nw *Noop) Warn(args ...interface{}) {
}

func (nw *Noop) Error(args ...interface{}) {
}

func (nw *Noop) WithContext(ctx context.Context) Logger {
	return nw
}
func (nw *Noop) WithError(err error) Logger {
	return nw
}

func (nw *Noop) WithField(key string, value interface{}) Logger {
	return nw
}

func (nw *Noop) WithFields(fields map[string]interface{}) Logger {
	return nw
}
