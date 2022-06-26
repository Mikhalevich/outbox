package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/errgroup"

	"github.com/Mikhalevich/outbox"
)

func main() {
	p, err := New()
	if err != nil {
		fmt.Printf("create postgre database connection: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()

	if err := p.CreateSchema(); err != nil {
		fmt.Printf("create schema error: %v\n", err)
		os.Exit(1)
	}

	o, err := outbox.New(p.db, outbox.Processors{
		"test": func(url string, payload string) error {
			fmt.Printf("send message for url: %s; msg: %v\n", url, payload)
			return nil
		},
	}, outbox.WithDispatcherCount(5))
	if err != nil {
		fmt.Printf("init outbox error: %v\n", err)
		os.Exit(1)
	}

	group, ctx := errgroup.WithContext(context.Background())

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

				count++
			}
		}
	})

	if err := group.Wait(); err != nil {
		fmt.Printf("wait group error: %v\n", err)
		os.Exit(1)
	}

	<-waitChan

	fmt.Println("done...")
}
