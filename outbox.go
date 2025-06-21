package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/Mikhalevich/outbox/pkg/logger"
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
type EventStorage interface {
	CreateSchema(ctx context.Context) error
	Insert(ctx context.Context, tx *sqlx.Tx, event Event) error
	Process(ctx context.Context, limit int, eventsFn EventProcessorFn) error
}

// Outbox structure.
type Outbox struct {
	storage   EventStorage
	processor EventProcessorFn
	opts      options
}

// New constructs new outbox instance.
func New(
	storage EventStorage,
	processor EventProcessorFn,
	opts ...Option,
) (*Outbox, error) {
	defaultOpts := options{
		DispatcherCount:  defaultDispatcherCount,
		BatchSize:        defaultBatchSize,
		DispatchInterval: defaultDispatchInterval,
		Logger:           logger.NewNullWrapper(),
	}

	for _, o := range opts {
		o(&defaultOpts)
	}

	outb := &Outbox{
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
func (o *Outbox) Send(ctx context.Context, tx *sqlx.Tx, queueURL string, payloadType string, payload []byte) error {
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
func (o *Outbox) SendJSON(
	ctx context.Context,
	trx *sqlx.Tx,
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
func (o *Outbox) Run(ctx context.Context) {
	var wGroup sync.WaitGroup

	wGroup.Add(o.opts.DispatcherCount)

	for dispatcherNum := range o.opts.DispatcherCount {
		go func(dispatcherNum int) {
			defer wGroup.Done()

			log := o.opts.Logger.WithContext(ctx).WithField("dispatcher_num", dispatcherNum)
			log.Info("outbox dispatcher is running")
			defer log.Info("outbox dispatcher stopped")

			o.runDispatcher(ctx, log)
		}(dispatcherNum)
	}

	wGroup.Wait()
}

// GoRun start event dispatch cycle in separate goroutine.
// returns channel specifying end of working processing for gracifull shutdown.
func (o *Outbox) GoRun(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		o.Run(ctx)
	}()

	return done
}

func (o *Outbox) runDispatcher(ctx context.Context, log logger.Logger) {
	ticker := time.NewTicker(o.opts.DispatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := o.dispatchEvents(ctx, log); err != nil {
				log.WithError(err).Error("outbox dispatch error")
			}
		}
	}
}

func (o *Outbox) dispatchEvents(ctx context.Context, log logger.Logger) error {
	if err := o.storage.Process(ctx, o.opts.BatchSize,
		func(events []Event) ([]EventID, error) {
			ids, err := o.processor(events)
			if err != nil {
				log.WithError(err).Error("process outbox events error")
				// skip error
			}

			if len(ids) > 0 {
				log.WithField("events_count", len(ids)).Debug("messages processed")
			}

			return ids, nil
		}); err != nil {
		return fmt.Errorf("storage process: %w", err)
	}

	return nil
}
