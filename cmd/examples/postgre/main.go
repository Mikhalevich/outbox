package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Mikhalevich/outbox"
)

func main() {
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

	o, err := outbox.New(p.db, outbox.Processors{
		"test": func(url string, payload string) error {
			logrus.Infof("<----- send message for url: %s; msg: %v\n", url, payload)
			return nil
		},
	}, outbox.WithDispatcherCount(1), outbox.WithDispatchInterval(time.Second*5))
	if err != nil {
		fmt.Printf("init outbox error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)

	waitChan := o.Run(ctx)

	group.Go(func() error {
		ticker := time.NewTicker(time.Second * 1)
		defer ticker.Stop()

		count := 0
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := p.InsertTest(&TestData{
					ID:        count,
					IntVal:    count,
					StringVal: fmt.Sprintf("string value: %d", count),
				}, func(tx *sqlx.Tx) error {
					return o.SendJSON(ctx, tx, "queueURL", "test", fmt.Sprintf("string value: %d", count))
				}); err != nil {
					return fmt.Errorf("insert test error: %w", err)
				}

				logrus.Info("insert message to db --------->")

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
