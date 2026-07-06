package flusher

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
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

	// maxBufferedEvents caps how many events may sit in the in-memory buffer.
	// When ingestion fails, unsent events are requeued for a later attempt;
	// without a cap, a sustained ingest outage on a high-volume function grows the
	// buffer without bound and the extension leaks memory until it hits the Lambda
	// memory ceiling (see
	// https://github.com/axiomhq/axiom-lambda-extension/issues/48). Beyond the cap
	// the oldest events are dropped. Override with AXIOM_MAX_BUFFERED_EVENTS.
	maxBufferedEvents = 100_000
)

func init() {
	logger, _ = zap.NewProduction()

	if v := os.Getenv("AXIOM_MAX_BUFFERED_EVENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxBufferedEvents = n
		} else {
			logger.Warn("invalid AXIOM_MAX_BUFFERED_EVENTS, using default",
				zap.String("value", v), zap.Int("default", maxBufferedEvents))
		}
	}
}

// ingester is the subset of *axiom.Client the flusher depends on. Depending on an
// interface rather than the concrete client lets the flush path be unit-tested and
// makes explicit that a flush honours the caller's context for cancellation.
//
// We use Ingest (a plain io.Reader body) rather than IngestEvents on purpose.
// IngestEvents streams events through an io.Pipe fed by a background zstd encoder
// goroutine; if the HTTP request is cancelled or stalls mid-flight, that goroutine
// blocks forever in Encoder.Close -> PipeWriter.Write and leaks itself plus its
// ~4MB compression buffer on every stalled flush. That is the memory leak in
// https://github.com/axiomhq/axiom-lambda-extension/issues/48. Encoding the body
// into a bytes buffer ourselves (see encodeBatch) has no background goroutine and
// nothing that can be stranded.
type ingester interface {
	Ingest(ctx context.Context, id string, r io.Reader, typ axiom.ContentType, enc axiom.ContentEncoding, options ...ingest.Option) (*ingest.Status, error)
}

type Axiom struct {
	client        ingester
	retryClient   ingester
	events        []axiom.Event
	eventsLock    sync.Mutex
	lastFlushTime time.Time
}

func New() (*Axiom, error) {
	// We create two almost identical clients, but one will retry and one will
	// not. This is mostly because we are just waiting for the next flush with
	// the next event most of the time, but want to retry on exit/shutdown.

	opts := make([]axiom.Option, 0, 3)
	opts = append(opts,
		axiom.SetAPITokenConfig(axiomToken),
		axiom.SetUserAgent(fmt.Sprintf("axiom-lambda-extension/%s", version.Get())),
	)

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

// Flush sends the buffered events to Axiom. The provided context bounds the
// ingest call: when it is cancelled (e.g. the per-invocation deadline is reached),
// the in-flight request is aborted so the extension can hand control back to the
// Lambda runtime instead of holding the sandbox open until the function times out
// (see issue #48). On failure the batch is requeued for a later attempt, bounded
// by maxBufferedEvents.
func (f *Axiom) Flush(ctx context.Context, opt RetryOpt) {
	f.eventsLock.Lock()
	var batch []axiom.Event
	// create a copy of the batch, clear the original
	batch, f.events = f.events, []axiom.Event{}
	f.lastFlushTime = time.Now()
	f.eventsLock.Unlock()

	if len(batch) == 0 {
		return
	}

	body, err := encodeBatch(batch)
	if err != nil {
		// Encoding failure is not transient, but requeue (bounded) so a later
		// flush can retry rather than silently dropping the batch.
		logger.Error("Failed to encode events", zap.Error(err))
		f.requeue(batch)
		return
	}

	var res *ingest.Status
	if opt == Retry {
		res, err = f.retryClient.Ingest(ctx, axiomDataset, body, axiom.NDJSON, axiom.Gzip)
	} else {
		res, err = f.client.Ingest(ctx, axiomDataset, body, axiom.NDJSON, axiom.Gzip)
	}

	if err != nil {
		if opt == Retry {
			logger.Error("Failed to ingest events", zap.Error(err))
		} else {
			logger.Error("Failed to ingest events (will try again with next event)", zap.Error(err))
		}
		// Allow this batch to be retried again by putting it back in front of any
		// events queued since, keeping the buffer bounded.
		f.requeue(batch)

		return
	} else if res.Failed > 0 {
		log.Printf("%d failures during ingesting, %s", res.Failed, res.Failures[0].Error)
	}
}

// encodeBatch serialises events as gzip-compressed NDJSON into an in-memory
// buffer. Building the body here (instead of via IngestEvents' streaming io.Pipe)
// is what makes the flush leak-free: a bytes.Buffer never blocks, so the gzip
// writer always closes cleanly and no encoder goroutine can be stranded when a
// flush is cancelled (see issue #48 and the ingester doc above). The returned
// *bytes.Reader lets net/http rewind the body for the shutdown retry path.
func encodeBatch(batch []axiom.Event) (*bytes.Reader, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	enc := json.NewEncoder(gz)
	for i := range batch {
		if err := enc.Encode(batch[i]); err != nil {
			_ = gz.Close()
			return nil, err
		}
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

// requeue puts a failed batch back at the front of the buffer, dropping the
// oldest events when the buffer would exceed maxBufferedEvents. Dropping oldest
// (rather than rejecting new) keeps the most recent logs, and copying into a
// right-sized slice releases the dropped events' backing array to the GC so a
// sustained outage cannot grow memory without bound (issue #48).
func (f *Axiom) requeue(batch []axiom.Event) {
	f.eventsLock.Lock()
	combined := append(batch, f.events...)
	dropped := 0
	if len(combined) > maxBufferedEvents {
		dropped = len(combined) - maxBufferedEvents
		trimmed := make([]axiom.Event, maxBufferedEvents)
		copy(trimmed, combined[dropped:])
		combined = trimmed
	}
	f.events = combined
	f.eventsLock.Unlock()

	if dropped > 0 {
		logger.Warn("event buffer full; dropped oldest events to bound memory (issue #48)",
			zap.Int("dropped", dropped),
			zap.Int("max_buffered_events", maxBufferedEvents))
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
