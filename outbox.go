package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Mikhalevich/outbox/logger"
)

const (
	defaultDispatcherCount  = 1
	defaultBatchSize        = 100
	defaultDispatchInterval = time.Second * 1
)

type EventID int64

func (e EventID) Int64() int64 {
	return int64(e)
}

// Event represents single event to proceed.
type Event struct {
	ID          EventID
	URL         string
	PayloadType string
	Payload     []byte
}

// EventProcessorFn custom processor event func.
// this func will be called by outbox dispatcher with batch of events.
// returns slice of processed event ids and error.
type EventProcessorFn func(events []Event) ([]EventID, error)

// CollectAllEventIDs helper wrapper around EventProcessorFn.
// for processing events and collecting all event ids.
func CollectAllEventIDs(processFn func(events []Event) error) EventProcessorFn {
	return func(events []Event) ([]EventID, error) {
		if err := processFn(events); err != nil {
			return nil, fmt.Errorf("process fn: %w", err)
		}

		ids := make([]EventID, 0, len(events))

		for _, event := range events {
			ids = append(ids, event.ID)
		}

		return ids, nil
	}
}

// EventStorage interface for external implementation for store and receive events from storage.
type EventStorage[T any] interface {
	CreateSchema(ctx context.Context) error
	Insert(ctx context.Context, tx T, event Event) error
	Process(ctx context.Context, limit int, eventsFn EventProcessorFn) error
}

// Outbox structure.
type Outbox[T any] struct {
	storage   EventStorage[T]
	processor EventProcessorFn
	opts      options
}

// New constructs new outbox instance.
func New[T any](
	storage EventStorage[T],
	processor EventProcessorFn,
	opts ...Option,
) (*Outbox[T], error) {
	defaultOpts := options{
		DispatcherCount:  defaultDispatcherCount,
		BatchSize:        defaultBatchSize,
		DispatchInterval: defaultDispatchInterval,
		Logger:           logger.NewNoop(),
	}

	for _, o := range opts {
		o(&defaultOpts)
	}

	outb := &Outbox[T]{
		storage:   storage,
		processor: processor,
		opts:      defaultOpts,
	}

	if err := outb.storage.CreateSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("create schema error: %w", err)
	}

	return outb, nil
}

// Send store single event with custom payload in external event storage.
func (o *Outbox[T]) Send(ctx context.Context, tx T, queueURL string, payloadType string, payload []byte) error {
	if err := o.storage.Insert(ctx, tx, Event{
		URL:         queueURL,
		PayloadType: payloadType,
		Payload:     payload,
	}); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

// SendJSON store single event with json payload in external event storage.
func (o *Outbox[T]) SendJSON(
	ctx context.Context,
	trx T,
	queueURL string,
	payloadType string,
	payload interface{},
) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	if err := o.Send(ctx, trx, queueURL, payloadType, b); err != nil {
		return fmt.Errorf("json send: %w", err)
	}

	return nil
}

// Run start event dispatch cycle.
func (o *Outbox[T]) Run(ctx context.Context) {
	var wGroup sync.WaitGroup

	wGroup.Add(o.opts.DispatcherCount)

	for dispatcherNum := range o.opts.DispatcherCount {
		go func(dispatcherNum int) {
			defer wGroup.Done()

			log := o.opts.Logger.WithContext(ctx).WithField("dispatcher_num", dispatcherNum)
			log.Info("outbox dispatcher is running")
			defer log.Info("outbox dispatcher stopped")

			o.runDispatcher(logger.WithLogger(ctx, log))
		}(dispatcherNum)
	}

	wGroup.Wait()
}

// GoRun start event dispatch cycle in separate goroutine.
// returns channel specifying end of working processing for gracifull shutdown.
func (o *Outbox[T]) GoRun(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		o.Run(ctx)
	}()

	return done
}

func (o *Outbox[T]) runDispatcher(ctx context.Context) {
	ticker := time.NewTicker(o.opts.DispatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := o.dispatchEvents(ctx); err != nil {
				logger.FromContext(ctx).
					WithError(err).
					Error("outbox dispatch error")
			}
		}
	}
}

func (o *Outbox[T]) dispatchEvents(ctx context.Context) error {
	if err := o.storage.Process(ctx, o.opts.BatchSize,
		func(events []Event) ([]EventID, error) {
			ids, err := o.processor(events)
			if err != nil {
				logger.FromContext(ctx).
					WithError(err).
					Error("process outbox events error")
				// skip error
			}

			if len(ids) > 0 {
				logger.FromContext(ctx).
					WithField("events_count", len(ids)).
					Debug("events processed")
			}

			return ids, nil
		}); err != nil {
		return fmt.Errorf("storage process: %w", err)
	}

	return nil
}
