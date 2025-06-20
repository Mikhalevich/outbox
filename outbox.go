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
	defaultBatchSize = 100
)

type ProcessorFunc func(queueURL string, payloadType string, payload []byte) error

type storager interface {
	CreateSchema(ctx context.Context) error
	Add(ctx context.Context, tx *sqlx.Tx, msg *storage.Message) error
	Process(ctx context.Context, limit int, fn storage.ProcessFunc) error
}

type Outbox struct {
	storage   storager
	processor ProcessorFunc
	opts      options
}

func New(sqlDB *sqlx.DB, processor ProcessorFunc, opts ...option) (*Outbox, error) {
	defaultOpts := options{
		DispatcherCount:  1,
		ButchSize:        defaultBatchSize,
		DispatchInterval: time.Second * 1,
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
	if err := o.storage.Add(ctx, tx, &storage.Message{
		QueueURL:    queueURL,
		PayloadType: payloadType,
		Payload:     payload,
	}); err != nil {
		return fmt.Errorf("add message error: %w", err)
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
		return fmt.Errorf("send json: marshal error: %w", err)
	}

	if err := o.Send(ctx, trx, queueURL, payloadType, b); err != nil {
		return fmt.Errorf("send json: %w", err)
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
	if err := o.storage.Process(ctx, o.opts.ButchSize, func(messages []storage.Message) ([]int, error) {
		ids := make([]int, 0, len(messages))

		for _, msg := range messages {
			if err := o.processor(msg.QueueURL, msg.PayloadType, msg.Payload); err != nil {
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
