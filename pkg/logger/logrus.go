package logger

import (
	"context"

	"github.com/sirupsen/logrus"
)

type logrusWrapper struct {
	l *logrus.Entry
}

func NewLogrusWrapper(log *logrus.Logger) *logrusWrapper {
	return &logrusWrapper{
		l: logrus.NewEntry(log),
	}
}

func (lw *logrusWrapper) Debug(args ...interface{}) {
	lw.l.Debug(args...)
}

func (lw *logrusWrapper) Info(args ...interface{}) {
	lw.l.Info(args...)
}

func (lw *logrusWrapper) Warn(args ...interface{}) {
	lw.l.Warn(args...)
}

func (lw *logrusWrapper) Error(args ...interface{}) {
	lw.l.Error(args...)
}

func (lw *logrusWrapper) WithContext(ctx context.Context) Logger {
	return &logrusWrapper{
		l: lw.l.WithContext(ctx),
	}
}
func (lw *logrusWrapper) WithError(err error) Logger {
	return &logrusWrapper{
		l: lw.l.WithError(err),
	}
}

func (lw *logrusWrapper) WithField(key string, value interface{}) Logger {
	return &logrusWrapper{
		l: lw.l.WithField(key, value),
	}
}

func (lw *logrusWrapper) WithFields(fields map[string]interface{}) Logger {
	return &logrusWrapper{
		l: lw.l.WithFields(fields),
	}
}
