package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

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

// lambda environment variables
var (
	AWS_LAMBDA_FUNCTION_NAME                  = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	AWS_REGION                                = os.Getenv("AWS_REGION")
	AWS_LAMBDA_FUNCTION_VERSION               = os.Getenv("AWS_LAMBDA_FUNCTION_VERSION")
	AWS_LAMBDA_INITIALIZATION_TYPE            = os.Getenv("AWS_LAMBDA_INITIALIZATION_TYPE")
	AWS_LAMBDA_FUNCTION_MEMORY_SIZE_STR       = os.Getenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE")
	AWS_LAMBDA_FUNCTION_MEMORY_SIZE     int64 = 0 // parse the memory size string in the Start method
	lambdaMetaInfo                            = map[string]any{}
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
	memSize, err := strconv.ParseInt(AWS_LAMBDA_FUNCTION_MEMORY_SIZE_STR, 10, 64)
	if err != nil {
		logger.Warn("Failed to parse lambda memory size", zap.Error(err))
	} else {
		AWS_LAMBDA_FUNCTION_MEMORY_SIZE = memSize
	}

	// initialize the lambdaMetaInfo map
	lambdaMetaInfo = map[string]any{
		"initializationType": AWS_LAMBDA_INITIALIZATION_TYPE,
		"region":             AWS_REGION,
		"name":               AWS_LAMBDA_FUNCTION_NAME,
		"memorySizeMB":       AWS_LAMBDA_FUNCTION_MEMORY_SIZE,
		"version":            AWS_LAMBDA_FUNCTION_VERSION,
	}
}

func (s *Server) httpHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("Error reading body:", zap.Error(err))
		return
	}

	var events []axiom.Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		return
	}

	for _, e := range events {
		// attach the lambda information to the event
		e["lambda"] = lambdaMetaInfo
		// replace the time field with axiom's _time
		e["_time"], e["time"] = e["time"], nil
	}

	_, err = s.axiomClient.IngestEvents(r.Context(), s.axiomDataset, events)
	if err != nil {
		logger.Error("Ingesting Events to Axiom Failed:", zap.Error(err))
		return
	}
}
