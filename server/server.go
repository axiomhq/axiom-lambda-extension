package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/axiomhq/axiom-go/axiom"
	version "github.com/axiomhq/axiom-lambda-extension/version"
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
	AWS_LAMBDA_FUNCTION_NAME        = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	AWS_REGION                      = os.Getenv("AWS_REGION")
	AWS_LAMBDA_FUNCTION_VERSION     = os.Getenv("AWS_LAMBDA_FUNCTION_VERSION")
	AWS_LAMBDA_INITIALIZATION_TYPE  = os.Getenv("AWS_LAMBDA_INITIALIZATION_TYPE")
	AWS_LAMBDA_FUNCTION_MEMORY_SIZE = os.Getenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE")
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
		logger.Error("Error reading body:", zap.Error(err))
		return
	}

	var events []axiom.Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		return
	}

	lambdaInfo := map[string]string{
		"initializationType": AWS_LAMBDA_INITIALIZATION_TYPE,
		"region":             AWS_REGION,
		"name":               AWS_LAMBDA_FUNCTION_NAME,
		"memorySizeMB":       AWS_LAMBDA_FUNCTION_MEMORY_SIZE,
		"version":            AWS_LAMBDA_FUNCTION_MEMORY_SIZE,
	}

	extVersion := map[string]string{
		"awsLambdaExtensionVersion": version.Get(),
	}

	for _, e := range events {
		e["lambda"] = lambdaInfo
		e["axiom"] = extVersion
	}

	_, err = s.axiomClient.IngestEvents(r.Context(), s.axiomDataset, events)
	if err != nil {
		logger.Error("Ingesting Events to Axiom Failed:", zap.Error(err))
		return
	}
}
