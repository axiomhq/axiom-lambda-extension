package flusher

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/axiomhq/axiom-lambda-extension/version"

	"github.com/axiomhq/axiom-go/axiom"
)

// Axiom Config
var (
	axiomToken    = os.Getenv("AXIOM_TOKEN")
	axiomDataset  = os.Getenv("AXIOM_DATASET")
	batchSize     = 1000
	flushInterval = 1 * time.Second
)

type Axiom struct {
	client    *axiom.Client
	EventChan chan axiom.Event
	batch     []axiom.Event
	ticker    *time.Ticker
}

func New() (*Axiom, error) {
	client, err := axiom.NewClient(
		axiom.SetAPITokenConfig(axiomToken),
		axiom.SetUserAgent(fmt.Sprintf("axiom-lambda-extension/%s", version.Get())),
		axiom.SetNoRetry(),
	)
	if err != nil {
		return nil, err
	}

	f := &Axiom{
		client:    client,
		EventChan: make(chan axiom.Event),
		batch:     make([]axiom.Event, 0, batchSize),
		ticker:    time.NewTicker(flushInterval),
	}

	ctx := context.Background()

	go func() {
		defer f.ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				f.ticker.Stop()
				return
			case event, ok := <-f.EventChan:
				if !ok {
					// channel is closed
					f.Flush()
					return
				}

				f.batch = append(f.batch, event)
				if len(f.batch) >= batchSize {
					f.Flush()
				}
			case <-f.ticker.C:
				f.Flush()
			}
		}
	}()

	return f, nil
}

func (f *Axiom) Flush() {
	if len(f.batch) == 0 {
		return
	}

	res, err := f.client.IngestEvents(context.Background(), axiomDataset, f.batch)
	if err != nil {
		log.Println(fmt.Errorf("failed to ingest events: %w", err))
		// allow this batch to be retried again
		f.ticker.Reset(flushInterval)
		return
	} else if res.Failed > 0 {
		log.Printf("%d failures during ingesting, %s", res.Failed, res.Failures[0].Error)
	}
	f.ticker.Reset(flushInterval)
	f.batch = f.batch[:0] // Clear the batch.
}
