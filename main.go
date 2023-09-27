package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/peterbourgon/ff/v2/ffcli"
	"go.uber.org/zap"

	"github.com/axiomhq/axiom-lambda-extension/extension"
	"github.com/axiomhq/axiom-lambda-extension/flusher"
	"github.com/axiomhq/axiom-lambda-extension/logsapi"
	"github.com/axiomhq/axiom-lambda-extension/server"
)

var (
	runtimeAPI        = os.Getenv("AWS_LAMBDA_RUNTIME_API")
	extensionName     = filepath.Base(os.Args[0])
	isFirstInvocation = true
	runtimeDone       = make(chan struct{})

	// API Port
	logsPort = "8080"

	// Buffering Config
	defaultMaxItems  = 1000
	defaultMaxBytes  = 262144
	defaultTimeoutMS = 1000

	developmentMode = false
	logger          *zap.Logger
)

func init() {
	logger, _ = zap.NewProduction()
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
		logger.Error("error creating axiom client", zap.Error(err))
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
	logsClient := logsapi.New(runtimeAPI)

	destination := logsapi.Destination{
		Protocol:   "HTTP",
		URI:        logsapi.URI(fmt.Sprintf("http://sandbox.localdomain:%s/", logsPort)),
		HttpMethod: "POST",
		Encoding:   "JSON",
	}

	bufferingCfg := logsapi.BufferingCfg{
		MaxItems:  uint32(defaultMaxItems),
		MaxBytes:  uint32(defaultMaxBytes),
		TimeoutMS: uint32(defaultTimeoutMS),
	}

	_, err = logsClient.Subscribe(ctx, []string{"function", "platform"}, bufferingCfg, destination, extensionClient.ExtensionID)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			flusher.SafelyUseAxiomClient(axiom, func(client *flusher.Axiom) {
				client.Flush()
			})
			logger.Info("Context Done", zap.Any("ctx", ctx.Err()))
			return nil
		default:
			res, err := extensionClient.NextEvent(ctx, extensionName)
			if err != nil {
				logger.Error("Next event Failed:", zap.Error(err))
				return err
			}

			// on every event received, check if we should flush
			flusher.SafelyUseAxiomClient(axiom, func(client *flusher.Axiom) {
				if client.ShouldFlush() {
					client.Flush()
				}
			})

			// wait for the first invocation to finish (receive platform.runtimeDone log), then flush
			if isFirstInvocation {
				<-runtimeDone
				isFirstInvocation = false
				flusher.SafelyUseAxiomClient(axiom, func(client *flusher.Axiom) {
					client.Flush()
				})
			}

			if res.EventType == "SHUTDOWN" {
				flusher.SafelyUseAxiomClient(axiom, func(client *flusher.Axiom) {
					client.Flush()
				})
				_ = httpServer.Shutdown()
				return nil
			}
		}
	}
}
