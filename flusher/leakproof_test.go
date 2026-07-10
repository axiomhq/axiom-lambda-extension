package flusher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
)

// TestFlushDoesNotStrandStreamingEncoder guards the fix for issue #48. The old
// path (axiom-go IngestEvents) streamed events through an io.Pipe fed by a
// background zstd encoder goroutine; a cancelled or stalled flush left that
// goroutine blocked forever in Encoder.Close -> PipeWriter.Write, leaking it and
// its ~4MB compression buffer on every flush. The in-memory gzip Ingest path used
// by the flusher must spawn no such goroutine. We drive many flushes against a
// server that stalls until the client cancels, then assert no zstd / io.Pipe
// goroutines remain.
func TestFlushDoesNotStrandStreamingEncoder(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done(): // client cancelled the request (stalled ingest)
		case <-release: // test teardown
		}
	}))
	// close(release) runs before srv.Close() (LIFO) so handlers exit first.
	defer srv.Close()
	defer close(release)

	client, err := axiom.NewClient(
		axiom.SetNoEnv(),
		axiom.SetURL(srv.URL),
		axiom.SetToken("xaat-00000000-0000-0000-0000-000000000000"),
		axiom.SetNoRetry(),
		axiom.SetNoTracing(),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	f := &Axiom{client: client, retryClient: client, events: make([]axiom.Event, 0)}

	const iterations = 50
	for i := 0; i < iterations; i++ {
		f.QueueEvents([]axiom.Event{{"i": i, "msg": "leak guard"}})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		f.Flush(ctx, NoRetry)
		cancel()
	}

	// Give any goroutine that was going to exit the chance to do so.
	for i := 0; i < 20; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}

	if n := countGoroutines("klauspost/compress/zstd", "io.(*pipe).write"); n != 0 {
		t.Fatalf("leaked %d streaming-encoder goroutine(s) after %d cancelled flushes; "+
			"ingest is using the streaming pipe path (issue #48)", n, iterations)
	}
}

// countGoroutines returns the number of live goroutines whose stack contains any
// of the given substrings.
func countGoroutines(substrs ...string) int {
	buf := make([]byte, 1<<22)
	n := runtime.Stack(buf, true)
	count := 0
	for _, g := range strings.Split(string(buf[:n]), "\n\ngoroutine ") {
		for _, s := range substrs {
			if strings.Contains(g, s) {
				count++
				break
			}
		}
	}
	return count
}
