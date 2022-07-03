package outbox

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Mikhalevich/outbox/pkg/logger"
)

type options struct {
	DispatcherCount  int
	ButchSize        int
	DispatchInterval time.Duration
	Logger           logger.Logger
}

type option func(opts *options)

func WithDispatcherCount(count int) option {
	return func(opts *options) {
		opts.DispatcherCount = count
	}
}

func WithButchSize(size int) option {
	return func(opts *options) {
		opts.ButchSize = size
	}
}

func WithDispatchInterval(interval time.Duration) option {
	return func(opts *options) {
		opts.DispatchInterval = interval
	}
}

func WithLogger(logger logger.Logger) option {
	return func(opts *options) {
		opts.Logger = logger
	}
}

func WithLogrusLogger(log *logrus.Logger) option {
	return func(opts *options) {
		opts.Logger = logger.NewLogrusWrapper(log)
	}
}
