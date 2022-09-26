package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
)

type Server struct {
	httpServer   *http.Server
	axiomClient  *axiom.Client
	axiomDataset string
}

func New(port string, axClient *axiom.Client, axDataset string) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr: fmt.Sprintf(":%s", port),
		},
		axiomClient:  axClient,
		axiomDataset: axDataset,
	}
}

func (s *Server) Start() {
	http.HandleFunc("/", s.httpHandler)
}

func (s *Server) httpHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}

	var events []axiom.Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		return
	}

	res, err := s.axiomClient.Datasets.IngestEvents(context.Background(), s.axiomDataset, axiom.IngestOptions{}, events...)
	if err != nil {
		fmt.Println("Ingestion failed", err)
	}
	fmt.Println("res", res)
}

func (s *Server) Shutdown() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := s.httpServer.Shutdown(ctx)
		defer cancel()
		if err != nil {
			fmt.Errorf("Failed to shutdown http server gracefully %s", err)
		} else {
			s.httpServer = nil
		}
	}
}
