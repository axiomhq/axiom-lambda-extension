package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/peterbourgon/ff/v2/ffcli"
	"go.uber.org/zap"

	"github.com/axiomhq/axiom-lambda-extension/extension"
	"github.com/axiomhq/axiom-lambda-extension/flusher"
	"github.com/axiomhq/axiom-lambda-extension/server"
	"github.com/axiomhq/axiom-lambda-extension/telemetryapi"
)

var (
	runtimeAPI        = os.Getenv("AWS_LAMBDA_RUNTIME_API")
	crashOnAPIErr     = os.Getenv("PANIC_ON_API_ERR")
	extensionName     = filepath.Base(os.Args[0])
	isFirstInvocation = true
	runtimeDone       = make(chan struct{})

	// API Port
	logsPort = "8080"

	// Buffering Config
	defaultMaxItems  uint32 = 1000
	defaultMaxBytes  uint32 = 262144
	defaultTimeoutMS uint32 = 1000

	// flushTimeout bounds how long a single flush may run. It caps how long the
	// extension can hold the sandbox open after the runtime is done. Without it a
	// stalled ingest blocks the extension from calling NextEvent, so Lambda keeps
	// the sandbox alive (and billed) until the function timeout — reported as a
	// full-window `extensionOverhead` span (issue #48). Override with
	// AXIOM_FLUSH_TIMEOUT (a Go duration, e.g. "3s").
	flushTimeout = 5 * time.Second

	// flushSafetyMargin is reserved before the invocation deadline so the extension
	// always has time to call NextEvent before Lambda times the function out.
	flushSafetyMargin = 500 * time.Millisecond

	developmentMode = false
	logger          *zap.Logger
)

func init() {
	logger, _ = zap.NewProduction()

	if v := os.Getenv("AXIOM_FLUSH_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			flushTimeout = d
		} else {
			logger.Warn("invalid AXIOM_FLUSH_TIMEOUT, using default",
				zap.String("value", v), zap.Duration("default", flushTimeout))
		}
	}
}

func main() {
	rootCmd := &ffcli.Command{
		ShortUsage: "axiom-lambda-extension [flags]",
		ShortHelp:  "run axiom-lambda-extension",
		FlagSet:    flag.NewFlagSet("axiom-lambda-extension", flag.ExitOnError),
		Exec: func(ctx context.Context, args []string) error {
			return Run()
		},
	}

	rootCmd.FlagSet.BoolVar(&developmentMode, "development-mode", false, "Set development Mode")

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil && err != flag.ErrHelp {
		fmt.Fprintln(os.Stderr, err)
	}
}

func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	axiom, err := flusher.New()
	if err != nil {
		// We don't want to exit with error, so that the extensions doesn't crash and crash the main function with it.
		// so we continue even if Axiom client is nil
		logger.Error("Failed to create Axiom client, no logs will be sent to Axiom", zap.Error(err))
		// if users want to crash on error, they can set the PANIC_ON_API_ERROR env variable
		if crashOnAPIErr == "true" {
			return err
		}
	}

	httpServer := server.New(logsPort, axiom, runtimeDone)
	go httpServer.Run(ctx)

	var extensionClient *extension.Client

	if developmentMode {
		<-ctx.Done()
		return nil
	}

	// Extension API REGISTRATION on startup
	extensionClient = extension.New(runtimeAPI)

	_, err = extensionClient.Register(ctx, extensionName)
	if err != nil {
		return err
	}

	// LOGS API SUBSCRIPTION
	telemetryClient := telemetryapi.New(runtimeAPI)

	destination := telemetryapi.Destination{
		Protocol:   "HTTP",
		URI:        telemetryapi.URI(fmt.Sprintf("http://sandbox.localdomain:%s/", logsPort)),
		HttpMethod: "POST",
		Encoding:   "JSON",
	}

	bufferingCfg := telemetryapi.BufferingCfg{
		MaxItems:  defaultMaxItems,
		MaxBytes:  defaultMaxBytes,
		TimeoutMS: defaultTimeoutMS,
	}

	_, err = telemetryClient.Subscribe(ctx, []string{"function", "platform"}, bufferingCfg, destination, extensionClient.ExtensionID)
	if err != nil {
		return err
	}

	// Make sure we flush with retry on exit, bounded so shutdown can't hang.
	defer func() {
		flusher.SafelyUseAxiomClient(axiom, func(client *flusher.Axiom) {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), flushTimeout)
			defer cancel()
			client.Flush(shutdownCtx, flusher.Retry)
		})
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Context done", zap.Error(ctx.Err()))
			return nil
		default:
			res, err := extensionClient.NextEvent(ctx, extensionName)
			if err != nil {
				logger.Error("Next event failed:", zap.Error(err))
				return err
			}

			// On every event received, check if we should flush. The flush is
			// bounded by the invocation deadline so a slow or stalled ingest can
			// never hold the sandbox open until the function times out (issue #48).
			flushCtx, cancel := flushContext(ctx, res.DeadlineMs)
			flusher.SafelyUseAxiomClient(axiom, func(client *flusher.Axiom) {
				if client.ShouldFlush() {
					// No retry, we'll try again with the next event
					client.Flush(flushCtx, flusher.NoRetry)
				}
			})
			cancel()

			// Wait for the first invocation to finish (receive platform.runtimeDone log), then flush
			if isFirstInvocation {
				<-runtimeDone
				isFirstInvocation = false
				flushCtx, cancel := flushContext(ctx, res.DeadlineMs)
				flusher.SafelyUseAxiomClient(axiom, func(client *flusher.Axiom) {
					// No retry, we'll try again with the next event
					client.Flush(flushCtx, flusher.NoRetry)
				})
				cancel()
			}

			if res.EventType == "SHUTDOWN" {
				_ = httpServer.Shutdown()
				return nil
			}
		}
	}
}

// flushContext derives a context for a single flush. The flush is bounded by both
// flushTimeout and the current invocation's deadline (minus flushSafetyMargin) so
// that a slow or stalled ingest is abandoned in time for the extension to call
// NextEvent — preventing the sandbox from being held open (and billed) until the
// function timeout. deadlineMs is the epoch-millis deadline from the Extensions
// API NextEvent response; it is 0 when unknown, in which case only flushTimeout
// applies.
func flushContext(parent context.Context, deadlineMs int64) (context.Context, context.CancelFunc) {
	timeout := flushTimeout
	if deadlineMs > 0 {
		if remaining := time.Until(time.UnixMilli(deadlineMs)) - flushSafetyMargin; remaining < timeout {
			timeout = remaining
		}
	}
	return context.WithTimeout(parent, timeout)
}
