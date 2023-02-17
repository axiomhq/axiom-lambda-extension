package flusher

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/axiomhq/axiom-lambda-extension/version"
)

// Axiom Config
var (
	axiomToken   = os.Getenv("AXIOM_TOKEN")
	axiomDataset = os.Getenv("AXIOM_DATASET")
)

type Axiom struct {
	client    *axiom.Client
	EventChan chan axiom.Event
	StopChan  chan bool
}

func New() (*Axiom, error) {
	client, err := axiom.NewClient(
		axiom.SetAPITokenConfig(axiomToken),
		axiom.SetUserAgent(fmt.Sprintf("axiom-lambda-extension/%s", version.Get())),
	)

	f := &Axiom{
		client:    client,
		EventChan: make(chan axiom.Event),
		StopChan:  make(chan bool),
	}

	go func() {
		defer close(f.StopChan)

		_, err := f.client.IngestChannel(context.Background(), axiomDataset, f.EventChan)
		if err != nil {
			log.Printf("Error: Ingesting Events to Axiom Failed: %s", err.Error())
			return
		}
	}()

	return f, err
}
