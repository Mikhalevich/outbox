package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Mikhalevich/outbox"
)

const (
	testDataType = "test_data"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	p, err := New()
	if err != nil {
		logrus.WithError(err).Error("create postgre database connection")
		os.Exit(1)
	}
	defer p.Close()

	if err := p.CreateSchema(); err != nil {
		logrus.WithError(err).Error("create schema")
		os.Exit(1)
	}

	messageProcessor := func(url string, payloadType string, payload []byte) error {
		if payloadType != testDataType {
			return fmt.Errorf("invalid payload type: %s", payloadType)
		}

		var td TestData
		if err := json.Unmarshal(payload, &td); err != nil {
			return fmt.Errorf("unmarshal payload error: %w", err)
		}

		logrus.Infof("<----- send message for url: %s; msg: %v\n", url, td)
		return nil
	}

	o, err := outbox.New(
		p.db,
		messageProcessor,
		outbox.WithDispatcherCount(1),
		outbox.WithDispatchInterval(time.Second*5),
		outbox.WithLogrusLogger(logrus.StandardLogger()),
	)

	if err != nil {
		logrus.WithError(err).Error("init outbox")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	waitChan := o.Run(ctx)

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		ticker := time.NewTicker(time.Second * 1)
		defer ticker.Stop()

		count := 0
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				td := TestData{
					ID:        count,
					IntVal:    count,
					StringVal: fmt.Sprintf("string value: %d", count),
				}

				if err := p.InsertTest(&td, func(tx *sqlx.Tx) error {
					return o.SendJSON(ctx, tx, "<some_queue_url>", testDataType, &td)
				}); err != nil {
					return fmt.Errorf("insert test error: %w", err)
				}

				logrus.Info("message inserted --------->")

				count++
			}
		}
	})

	if err := group.Wait(); err != nil {
		logrus.WithError(err).Error("wait group")
		os.Exit(1)
	}

	<-waitChan

	logrus.Info("done...")
}
