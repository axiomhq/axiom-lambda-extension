package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/axiomhq/axiom-go/axiom"
	"go.uber.org/zap"

	"github.com/axiomhq/axiom-lambda-extension/version"
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
	AWS_LAMBDA_FUNCTION_NAME           = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	AWS_REGION                         = os.Getenv("AWS_REGION")
	AWS_LAMBDA_FUNCTION_VERSION        = os.Getenv("AWS_LAMBDA_FUNCTION_VERSION")
	AWS_LAMBDA_INITIALIZATION_TYPE     = os.Getenv("AWS_LAMBDA_INITIALIZATION_TYPE")
	AWS_LAMBDA_FUNCTION_MEMORY_SIZE, _ = strconv.ParseInt(os.Getenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE"), 10, 32)
	lambdaMetaInfo                     = map[string]any{}
	axiomMetaInfo                      = map[string]string{}
)

func init() {
	logger, _ = zap.NewProduction()

	// initialize the lambdaMetaInfo map
	lambdaMetaInfo = map[string]any{
		"initializationType": AWS_LAMBDA_INITIALIZATION_TYPE,
		"region":             AWS_REGION,
		"name":               AWS_LAMBDA_FUNCTION_NAME,
		"memorySizeMB":       AWS_LAMBDA_FUNCTION_MEMORY_SIZE,
		"version":            AWS_LAMBDA_FUNCTION_VERSION,
	}
	axiomMetaInfo = map[string]string{
		"awsLambdaExtensionVersion": version.Get(),
	}
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

func (s *Server) Shutdown(ctx context.Context) {
	s.httpServer.Shutdown(ctx)
}

func (s *Server) Start() {
	http.HandleFunc("/", s.httpHandler)

	err := s.httpServer.ListenAndServe()
	logger.Error("ListenAndServe returned an error", zap.Error(err))
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
		logger.Error("Error unmarshalling body:", zap.Error(err))
		return
	}

	for _, e := range events {
		// attach the lambda information to the event
		e["lambda"] = lambdaMetaInfo
		e["axiom"] = axiomMetaInfo
		// replace the time field with axiom's _time
		e["_time"], e["time"] = e["time"], nil
	}

	_, err = s.axiomClient.IngestEvents(context.Background(), s.axiomDataset, events)
	if err != nil {
		logger.Error("Ingesting Events to Axiom Failed:", zap.Error(err))
		return
	}

}
