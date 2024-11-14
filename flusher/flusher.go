package flusher

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

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
	axiomToken   = os.Getenv("AXIOM_TOKEN")
	axiomDataset = os.Getenv("AXIOM_DATASET")
	logger       *zap.Logger
)

func init() {
	logger, _ = zap.NewProduction()
}

type Axiom struct {
	client     *axiom.Client
	events     []axiom.Event
	eventsLock sync.Mutex
}

func New() (*Axiom, error) {
	// We create two almost identical clients, but one will retry and one will
	// not. This is mostly because we are just waiting for the next flush with
	// the next event most of the time, but want to retry on exit/shutdown.

	opts := []axiom.Option{
		axiom.SetAPITokenConfig(axiomToken),
		axiom.SetUserAgent(fmt.Sprintf("axiom-lambda-extension/%s", version.Get())),
	}

	opts = append(opts, axiom.SetNoRetry())
	client, err := axiom.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	f := &Axiom{
		client: client,
		events: make([]axiom.Event, 0),
	}

	return f, nil
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

func (f *Axiom) Flush() {
	f.eventsLock.Lock()
	var batch []axiom.Event
	// create a copy of the batch, clear the original
	batch, f.events = f.events, []axiom.Event{}
	f.eventsLock.Unlock()

	if len(batch) == 0 {
		return
	}

	var res *ingest.Status
	var err error
	res, err = f.client.IngestEvents(context.Background(), axiomDataset, batch)

	if err != nil {
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
