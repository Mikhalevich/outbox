package logger

import "context"

type nullWrapper struct {
}

func NewNullWrapper() *nullWrapper {
	return &nullWrapper{}
}

func (nw *nullWrapper) Debug(args ...interface{}) {
}

func (nw *nullWrapper) Info(args ...interface{}) {
}

func (nw *nullWrapper) Warn(args ...interface{}) {
}

func (nw *nullWrapper) Error(args ...interface{}) {
}

func (nw *nullWrapper) WithContext(ctx context.Context) Logger {
	return nw
}
func (nw *nullWrapper) WithError(err error) Logger {
	return nw
}

func (nw *nullWrapper) WithField(key string, value interface{}) Logger {
	return nw
}

func (nw *nullWrapper) WithFields(fields map[string]interface{}) Logger {
	return nw
}
