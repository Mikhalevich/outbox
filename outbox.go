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

type Event struct {
	URL         string
	PayloadType string
	Payload     []byte
}

type EventProcessorFn func(e Event) error

type EventStorage interface {
	CreateSchema(ctx context.Context) error
	Insert(ctx context.Context, tx *sqlx.Tx, msg *storage.Message) error
	Process(ctx context.Context, limit int, fn storage.ProcessFunc) error
}

type Outbox struct {
	storage   EventStorage
	processor EventProcessorFn
	opts      options
}

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

func (o *Outbox) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		var wGroup sync.WaitGroup

		wGroup.Add(o.opts.DispatcherCount)

		for workerNum := range o.opts.DispatcherCount {
			go func(workerNum int) {
				log := o.opts.Logger.WithContext(ctx).WithField("woker_num", workerNum)
				log.Info("run outbox dispatcher")
				defer log.Info("stop outbox dispatcher")

				defer wGroup.Done()
				o.runDispatcher(ctx, log)
			}(workerNum)
		}

		wGroup.Wait()
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
