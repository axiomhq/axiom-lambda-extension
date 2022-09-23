package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
)

type Server struct {
	httpServer   *http.Server
	axiomClient  *axiom.Client
	axiomDataset string
}

func New(port int, axClient *axiom.Client, axDataset string) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr: fmt.Sprintf(":%s", string(port)),
		},
		axiomClient:  axClient,
		axiomDataset: axDataset,
	}
}

func (s *Server) Start() {
	http.HandleFunc("/", s.httpHandler)
}

func (s *Server) httpHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	fmt.Println("received Logs:", string(body))

	var events []axiom.Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		fmt.Println("marshalling failed", err)
		return
	}
	fmt.Println(events)

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
