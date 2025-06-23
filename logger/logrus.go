package logger

import (
	"context"

	"github.com/sirupsen/logrus"
)

// Logrus is logrus implementation of Logger interface.
type Logrus struct {
	l *logrus.Entry
}

func NewLogrus(log *logrus.Logger) *Logrus {
	return &Logrus{
		l: logrus.NewEntry(log),
	}
}

func (lw *Logrus) Debug(args ...interface{}) {
	lw.l.Debug(args...)
}

func (lw *Logrus) Info(args ...interface{}) {
	lw.l.Info(args...)
}

func (lw *Logrus) Warn(args ...interface{}) {
	lw.l.Warn(args...)
}

func (lw *Logrus) Error(args ...interface{}) {
	lw.l.Error(args...)
}

func (lw *Logrus) WithContext(ctx context.Context) Logger {
	return &Logrus{
		l: lw.l.WithContext(ctx),
	}
}
func (lw *Logrus) WithError(err error) Logger {
	return &Logrus{
		l: lw.l.WithError(err),
	}
}

func (lw *Logrus) WithField(key string, value interface{}) Logger {
	return &Logrus{
		l: lw.l.WithField(key, value),
	}
}

func (lw *Logrus) WithFields(fields map[string]interface{}) Logger {
	return &Logrus{
		l: lw.l.WithFields(fields),
	}
}
