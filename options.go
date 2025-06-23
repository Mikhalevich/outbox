package outbox

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Mikhalevich/outbox/logger"
)

type options struct {
	DispatcherCount  int
	BatchSize        int
	DispatchInterval time.Duration
	Logger           logger.Logger
}

// Option represents specific outbox option.
type Option func(opts *options)

// WithDispatcherCount specifies polling workers number(1 by default).
func WithDispatcherCount(count int) Option {
	return func(opts *options) {
		opts.DispatcherCount = count
	}
}

// WithBatchSize specifies number of separate events for single worker processing(100 by default).
func WithBatchSize(size int) Option {
	return func(opts *options) {
		opts.BatchSize = size
	}
}

// WithDispatchInterval specifies time period of each worker execution(1 second by default).
func WithDispatchInterval(interval time.Duration) Option {
	return func(opts *options) {
		opts.DispatchInterval = interval
	}
}

// WithLogger set custom logger for outbox.
func WithLogger(logger logger.Logger) Option {
	return func(opts *options) {
		opts.Logger = logger
	}
}

// WithLogrusLogger set logrus logger for outbox.
func WithLogrusLogger(log *logrus.Logger) Option {
	return func(opts *options) {
		opts.Logger = logger.NewLogrus(log)
	}
}
