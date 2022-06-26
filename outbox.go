package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	"github.com/Mikhalevich/outbox/internal/storage"
	"github.com/Mikhalevich/outbox/internal/storage/postgre"
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

func New(db *sqlx.DB, processor ProcessorFunc, opts ...option) (*Outbox, error) {
	defaultOpts := options{
		DispatcherCount:  1,
		ButchSize:        100,
		DispatchInterval: time.Second * 1,
	}

	for _, o := range opts {
		o(&defaultOpts)
	}

	o := &Outbox{
		storage:   postgre.New(db),
		processor: processor,
		opts:      defaultOpts,
	}

	if err := o.storage.CreateSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("create schema error: %w", err)
	}

	return o, nil
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

func (o *Outbox) SendJSON(ctx context.Context, tx *sqlx.Tx, queueURL string, payloadType string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("send json: marshal error: %w", err)
	}

	if err := o.Send(ctx, tx, queueURL, payloadType, b); err != nil {
		return fmt.Errorf("send json: %w", err)
	}
	return nil
}

func (o *Outbox) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		wg.Add(o.opts.DispatcherCount)

		for i := 0; i < o.opts.DispatcherCount; i++ {
			go func(i int) {
				log := logrus.WithContext(ctx).WithField("woker_num", i)
				log.Info("run outbox dispatcher")
				defer log.Info("stop outbox dispatcher")

				defer wg.Done()
				o.runDispatcher(ctx, log)
			}(i)
		}

		wg.Wait()
	}()

	return done
}

func (o *Outbox) runDispatcher(ctx context.Context, log *logrus.Entry) {
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

func (o *Outbox) dispatch(ctx context.Context, log *logrus.Entry) error {
	return o.storage.Process(ctx, o.opts.ButchSize, func(messages []storage.Message) ([]int, error) {
		ids := make([]int, 0, len(messages))
		for _, m := range messages {
			if err := o.processor(m.QueueURL, m.PayloadType, m.Payload); err != nil {
				log.WithError(err).Error("process outbox message error")
				continue
			}

			ids = append(ids, m.ID)
		}

		return ids, nil
	})
}
