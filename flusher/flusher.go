package flusher

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/axiomhq/axiom-go/axiom/ingest"
	"go.uber.org/zap"

	"github.com/axiomhq/axiom-lambda-extension/version"
)

type RetryOpt int

const (
	NoRetry RetryOpt = iota
	Retry
)

// Axiom Config
var (
	axiomToken    = os.Getenv("AXIOM_TOKEN")
	axiomDataset  = os.Getenv("AXIOM_DATASET")
	batchSize     = 1000
	flushInterval = 1 * time.Second
	logger        *zap.Logger
)

func init() {
	logger, _ = zap.NewProduction()
}

type Axiom struct {
	client        *axiom.Client
	retryClient   *axiom.Client
	events        []axiom.Event
	eventsLock    sync.Mutex
	lastFlushTime time.Time
}

func New() (*Axiom, error) {
	// We create two almost identical clients, but one will retry and one will
	// not. This is mostly because we are just waiting for the next flush with
	// the next event most of the time, but want to retry on exit/shutdown.

	opts := []axiom.Option{
		axiom.SetAPITokenConfig(axiomToken),
		axiom.SetUserAgent(fmt.Sprintf("axiom-lambda-extension/%s", version.Get())),
	}

	retryClient, err := axiom.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	opts = append(opts, axiom.SetNoRetry())
	client, err := axiom.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	f := &Axiom{
		client:      client,
		retryClient: retryClient,
		events:      make([]axiom.Event, 0),
	}

	return f, nil
}

func (f *Axiom) ShouldFlush() bool {
	f.eventsLock.Lock()
	defer f.eventsLock.Unlock()

	return len(f.events) > batchSize || f.lastFlushTime.IsZero() || time.Since(f.lastFlushTime) > flushInterval
}

func (f *Axiom) Queue(event axiom.Event) {
	f.eventsLock.Lock()
	defer f.eventsLock.Unlock()

	f.events = append(f.events, event)
}

func (f *Axiom) QueueEvents(events []axiom.Event) {
	f.eventsLock.Lock()
	defer f.eventsLock.Unlock()

	f.events = append(f.events, events...)
}

func (f *Axiom) Flush(opt RetryOpt) {
	f.eventsLock.Lock()
	var batch []axiom.Event
	// create a copy of the batch, clear the original
	batch, f.events = f.events, []axiom.Event{}
	f.eventsLock.Unlock()

	f.lastFlushTime = time.Now()
	if len(batch) == 0 {
		return
	}

	var res *ingest.Status
	var err error
	if opt == Retry {
		res, _ = f.retryClient.IngestEvents(context.Background(), axiomDataset, batch)
	} else {
		res, _ = f.client.IngestEvents(context.Background(), axiomDataset, batch)
	}

	if err != nil {
		logger.Error("Failed to ingest events", zap.Error(err))
		// allow this batch to be retried again, put them back
		f.eventsLock.Lock()
		defer f.eventsLock.Unlock()
		f.events = append(batch, f.events...)

		return
	} else if res.Failed > 0 {
		log.Printf("%d failures during ingesting, %s", res.Failed, res.Failures[0].Error)
	}
}

// SafelyUseAxiomClient checks if axiom is empty, and if not, executes the given
func SafelyUseAxiomClient(axiom *Axiom, action func(*Axiom)) {
	if axiom != nil {
		action(axiom)
	} else {
		logger.Error("Attempted to use uninitialized Axiom client.")
	}
}
