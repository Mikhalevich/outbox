# outbox
Simple implementation of outbox pattern in Golang

###### Documentation 
[![Go Reference](https://pkg.go.dev/badge/github.com/Mikhalevich/outbox.svg)](https://pkg.go.dev/github.com/Mikhalevich/outbox)

### example
```golang
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
	"github.com/Mikhalevich/outbox/eventstorage/postgresqlx"
)

const (
	testDataType = "test_data"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	repo, err := NewPostgresRepository()
	if err != nil {
		logrus.WithError(err).Error("create postgre database connection")
		os.Exit(1)
	}
	defer repo.Close()

	if err := repo.CreateSchema(); err != nil {
		logrus.WithError(err).Error("create schema")
		os.Exit(1)
	}

	eventProcessor := func(events []outbox.Event) error {
		for _, event := range events {
			if event.PayloadType != testDataType {
				return fmt.Errorf("invalid payload type: %s", event.PayloadType)
			}

			var td TestData
			if err := json.Unmarshal(event.Payload, &td); err != nil {
				return fmt.Errorf("unmarshal payload error: %w", err)
			}

			logrus.Infof("<----- send message for url: %s; msg: %v\n", event.URL, td)
		}

		return nil
	}

	otbx := outbox.New(
		postgresqlx.New(repo.db),
		outbox.CollectAllEventIDs(eventProcessor),
		outbox.WithDispatcherCount(1),
		outbox.WithDispatchInterval(time.Second*5),
		outbox.WithLogrusLogger(logrus.StandardLogger()),
	)

	if err := otbx.CreateSchema(context.Background()); err != nil {
		logrus.WithError(err).Error("create schema")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

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

				if err := repo.InsertTest(&td, func(tx *sqlx.Tx) error {
					return otbx.SendJSON(ctx, tx, "<some_queue_url>", testDataType, &td)
				}); err != nil {
					return fmt.Errorf("insert test error: %w", err)
				}

				logrus.Info("message inserted --------->")

				count++
			}
		}
	})

	otbx.Run(ctx)

	if err := group.Wait(); err != nil {
		logrus.WithError(err).Error("wait group")
		os.Exit(1)
	}

	logrus.Info("done...")
}
```


## License

Outbox is released under the
[MIT License](http://www.opensource.org/licenses/MIT).
