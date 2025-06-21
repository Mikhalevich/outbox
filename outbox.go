package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/Mikhalevich/outbox/internal/storage"
	"github.com/Mikhalevich/outbox/internal/storage/postgre"
	"github.com/Mikhalevich/outbox/pkg/logger"
)

const (
	defaultDispatcherCount  = 1
	defaultBatchSize        = 100
	defaultDispatchInterval = time.Second * 1
)

// Event represents single event to proceed.
type Event struct {
	URL         string
	PayloadType string
	Payload     []byte
}

// EventProcessorFn custom processor event func.
// this func will be called by outbox dispatcher with batch of events.
type EventProcessorFn func(e Event) error

// EventStorage interface for external implementation for store and receive events from storage.
type EventStorage interface {
	CreateSchema(ctx context.Context) error
	Insert(ctx context.Context, tx *sqlx.Tx, msg *storage.Message) error
	Process(ctx context.Context, limit int, fn storage.ProcessFunc) error
}

// Outbox structure.
type Outbox struct {
	storage   EventStorage
	processor EventProcessorFn
	opts      options
}

// New constructs new outbox instance.
func New(
	sqlDB *sqlx.DB,
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
		storage:   postgre.New(sqlDB),
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
	if err := o.storage.Insert(ctx, tx, &storage.Message{
		QueueURL:    queueURL,
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
			if err := o.dispatch(ctx, log); err != nil {
				log.WithError(err).Error("outbox dispatch error")
			}
		}
	}
}

func (o *Outbox) dispatch(ctx context.Context, log logger.Logger) error {
	if err := o.storage.Process(ctx, o.opts.BatchSize, func(messages []storage.Message) ([]int, error) {
		ids := make([]int, 0, len(messages))

		for _, msg := range messages {
			if err := o.processor(
				Event{
					URL:         msg.QueueURL,
					PayloadType: msg.PayloadType,
					Payload:     msg.Payload,
				},
			); err != nil {
				log.WithError(err).Error("process outbox message error")

				continue
			}

			ids = append(ids, msg.ID)
		}

		log.WithField("message_count", len(ids)).Debug("messages processed")

		return ids, nil
	}); err != nil {
		return fmt.Errorf("storage process: %w", err)
	}

	return nil
}
