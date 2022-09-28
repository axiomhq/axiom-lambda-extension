package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/axiomhq/axiom-go/axiom"
	"go.uber.org/zap"
)

type Server struct {
	httpServer   *http.Server
	axiomClient  *axiom.Client
	axiomDataset string
}

var (
	logger *zap.Logger
)

func init() {
	logger, _ = zap.NewProduction()
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

	_ = s.httpServer.ListenAndServe()
}

func (s *Server) httpHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Info("Error reading body:", zap.Error(err))
	}

	var events []axiom.Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		return
	}

	res, err := s.axiomClient.Datasets.IngestEvents(context.Background(), s.axiomDataset, axiom.IngestOptions{}, events...)
	if err != nil {
		logger.Info("Ingesting Events to Axiom Failed:", zap.Error(err))
	}
	logger.Info("Ingesting Events to Axiom Succeeded:", zap.Any("response", res))
}
