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
	"github.com/axiomhq/axiom-lambda-extension/extension"
	"github.com/axiomhq/axiom-lambda-extension/logsapi"
	"github.com/axiomhq/axiom-lambda-extension/server"
	"github.com/peterbourgon/ff/v2/ffcli"
	"go.uber.org/zap"
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
	axiomURL     = os.Getenv("AXIOM_URL")
	axiomToken   = os.Getenv("AXIOM_TOKEN")
	axiomDataset = os.Getenv("AXIOM_DATASET")

	developmentMode = false
	logger          *zap.Logger
)

func init() {
	logger, _ = zap.NewProduction()
}

func main() {
	if axiomURL == "" {
		axiomURL = "https://cloud.axiom.co"
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	defer close(sigs)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		s := <-sigs
		cancel()
		logger.Info("Received", zap.Any("Signal", s))
		_ = logger.Sync()
		logger.Fatal("Exiting")
	}()

	rootCmd := &ffcli.Command{
		ShortUsage: "axiom-lambda-extension [flags]",
		ShortHelp:  "run axiom-lambda-extension", //TODO
		FlagSet:    flag.NewFlagSet("axiom-lambda-extension", flag.ExitOnError),
		Exec: func(ctx context.Context, args []string) error {
			return Run(ctx)
		},
	}

	rootCmd.FlagSet.BoolVar(&developmentMode, "development-mode", false, "Set development Mode")

	// rootCmd.Execute()
	if err := rootCmd.ParseAndRun(ctx, os.Args[1:]); err != nil && err != flag.ErrHelp {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Run(ctx context.Context) error {
	var extensionClient *extension.Client
	if !developmentMode {
		// Extension API REGISTRATION
		extensionClient = extension.New(runtimeAPI)

		_, err := extensionClient.Register(ctx, extensionName)
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

		res, err := logsClient.Subscribe(ctx, []string{"function", "platform"}, bufferingCfg, destination, extensionClient.ExtensionID)
		if err != nil {
			return err
		}
		logger.Info("Subscription Result", zap.Any("subscription", res))
	}

	axClient, _ := axiom.NewClient(
		axiom.SetURL(axiomURL),
		axiom.SetAccessToken(axiomToken),
	)
	server := server.New(logsPort, axClient, axiomDataset)
	server.Start()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if !developmentMode {
				nextEventResponse, err := extensionClient.NextEvent(ctx, extensionName)
				if err != nil {
					logger.Error("Next event Failed:", zap.Error(err))
					return err
				}
				logger.Info("Next Event Info", zap.Any("stats", nextEventResponse))
			}
		}
	}
}
