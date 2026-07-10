package flusher

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/axiomhq/axiom-go/axiom/ingest"
)

// fakeIngester is a test double for the ingester interface.
type fakeIngester struct {
	mu    sync.Mutex
	calls int
	err   error
	block bool // if true, block until the context is cancelled
}

func (f *fakeIngester) Ingest(ctx context.Context, _ string, r io.Reader, _ axiom.ContentType, _ axiom.ContentEncoding, _ ...ingest.Option) (*ingest.Status, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	// Drain the body as a real client would.
	_, _ = io.Copy(io.Discard, r)
	if f.err != nil {
		return nil, f.err
	}
	return &ingest.Status{}, nil
}

func (f *fakeIngester) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newTestAxiom(client ingester) *Axiom {
	return &Axiom{
		client:      client,
		retryClient: client,
		events:      make([]axiom.Event, 0),
	}
}

// bufferLen returns the number of buffered events. Test helper.
func (f *Axiom) bufferLen() int {
	f.eventsLock.Lock()
	defer f.eventsLock.Unlock()
	return len(f.events)
}

func TestFlushClearsBufferOnSuccess(t *testing.T) {
	fake := &fakeIngester{}
	f := newTestAxiom(fake)
	f.QueueEvents([]axiom.Event{{"a": 1}, {"b": 2}})

	f.Flush(context.Background(), NoRetry)

	if got := fake.callCount(); got != 1 {
		t.Fatalf("expected 1 ingest call, got %d", got)
	}
	if n := f.bufferLen(); n != 0 {
		t.Fatalf("expected empty buffer after successful flush, got %d", n)
	}
}

func TestFlushRequeuesOnError(t *testing.T) {
	fake := &fakeIngester{err: errors.New("boom")}
	f := newTestAxiom(fake)
	f.QueueEvents([]axiom.Event{{"a": 1}, {"b": 2}})

	f.Flush(context.Background(), NoRetry)

	if n := f.bufferLen(); n != 2 {
		t.Fatalf("expected 2 events requeued after failed flush, got %d", n)
	}
}

func TestFlushRequeueIsBounded(t *testing.T) {
	prev := maxBufferedEvents
	maxBufferedEvents = 3
	defer func() { maxBufferedEvents = prev }()

	fake := &fakeIngester{err: errors.New("boom")}
	f := newTestAxiom(fake)
	// Newer events are queued after the batch; the oldest must be dropped.
	f.QueueEvents([]axiom.Event{{"n": 0}, {"n": 1}, {"n": 2}, {"n": 3}, {"n": 4}})

	f.Flush(context.Background(), NoRetry)

	if n := f.bufferLen(); n != 3 {
		t.Fatalf("expected buffer capped at 3, got %d", n)
	}
	f.eventsLock.Lock()
	defer f.eventsLock.Unlock()
	if got := f.events[len(f.events)-1]["n"]; got != 4 {
		t.Fatalf("expected newest event retained at tail, got %v", got)
	}
	if got := f.events[0]["n"]; got != 2 {
		t.Fatalf("expected oldest retained to be n=2 after dropping 0 and 1, got %v", got)
	}
}

func TestFlushRespectsContextCancellation(t *testing.T) {
	fake := &fakeIngester{block: true}
	f := newTestAxiom(fake)
	f.QueueEvents([]axiom.Event{{"a": 1}})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		f.Flush(ctx, NoRetry)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Flush did not return after context cancellation")
	}

	// Events should be requeued for a later attempt, not lost.
	if n := f.bufferLen(); n != 1 {
		t.Fatalf("expected event requeued after cancelled flush, got %d", n)
	}
}

func TestFlushEmptyBufferIsNoop(t *testing.T) {
	fake := &fakeIngester{}
	f := newTestAxiom(fake)

	f.Flush(context.Background(), NoRetry)

	if got := fake.callCount(); got != 0 {
		t.Fatalf("expected no ingest call for empty buffer, got %d", got)
	}
}
