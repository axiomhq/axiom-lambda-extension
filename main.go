package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/peterbourgon/ff/v2/ffcli"
	"go.uber.org/zap"

	"github.com/axiomhq/axiom-lambda-extension/extension"
	"github.com/axiomhq/axiom-lambda-extension/logsapi"
	"github.com/axiomhq/axiom-lambda-extension/server"
	"github.com/axiomhq/axiom-lambda-extension/version"
)

var (
	runtimeAPI    = os.Getenv("AWS_LAMBDA_RUNTIME_API")
	extensionName = filepath.Base(os.Args[0])

	// API Port
	logsPort = "8080"

	// Buffering Config
	defaultMaxItems  = 1000
	defaultMaxBytes  = 262144
	defaultTimeoutMS = 1000

	// Axiom Config
	axiomToken   = os.Getenv("AXIOM_TOKEN")
	axiomDataset = os.Getenv("AXIOM_DATASET")

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
			return Run(ctx)
		},
	}

	rootCmd.FlagSet.BoolVar(&developmentMode, "development-mode", false, "Set development Mode")

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil && err != flag.ErrHelp {
		fmt.Fprintln(os.Stderr, err)
		// TODO: we don't exist here so that we don't kill the lambda
		// os.Exit(1)
	}
}

func Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	axClient, err := axiom.NewClient(
		axiom.SetAPITokenConfig(axiomToken),
		axiom.SetUserAgent(fmt.Sprintf("axiom-lambda-extension/%s", version.Get())),
	)
	if err != nil {
		return err
	}

	httpServer := server.New(logsPort, axClient, axiomDataset)
	go httpServer.Start()

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
			logger.Info("Context Done", zap.Any("ctx", ctx.Err()))
			stop()
			return nil
		default:
			res, err := extensionClient.NextEvent(ctx, extensionName)
			if err != nil {
				logger.Error("Next event Failed:", zap.Error(err))
				return err
			}

			if res.EventType == "SHUTDOWN" {
				httpServer.Shutdown(ctx)
				cancel()
				return nil
			}
		}
	}
}
