package outbox

import (
	"time"
)

type options struct {
	DispatcherCount  int
	ButchSize        int
	DispatchInterval time.Duration
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
