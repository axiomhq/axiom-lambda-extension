package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"os"
	"strconv"

	"go.uber.org/zap"

	"github.com/axiomhq/axiom-lambda-extension/version"

	"github.com/axiomhq/axiom-go/axiom"

	"github.com/axiomhq/axiom-lambda-extension/flusher"

	axiomHttp "github.com/axiomhq/pkg/http"
)

var (
	logger              *zap.Logger
	firstInvocationDone = false
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

var logLineRgx, _ = regexp.Compile(`^([0-9.:TZ-]{20,})\s+([0-9a-f-]{36})\s+(ERROR|INFO|WARN|DEBUG|TRACE)\s+(.*)`)

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

func New(port string, axiom *flusher.Axiom, runtimeDone chan struct{}) *axiomHttp.Server {
	s, err := axiomHttp.NewServer(fmt.Sprintf(":%s", port), httpHandler(axiom, runtimeDone))
	if err != nil {
		logger.Error("Error creating server", zap.Error(err))
		return nil
	}

	return s
}

func httpHandler(ax *flusher.Axiom, runtimeDone chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		notifyRuntimeDone := false

		for _, e := range events {
			// attach the lambda information to the event
			e["lambda"] = lambdaMetaInfo
			e["axiom"] = axiomMetaInfo
			// replace the time field with axiom's _time
			e["_time"], e["time"] = e["time"], nil

			if e["type"] == "function" {
				e["message"] = e["record"]
				if recordStr, ok := e["record"].(string); ok && len(recordStr) > 0 {
					recordStr = strings.Trim(recordStr, "\n")
					// parse the record
					// first check if the record is a json object, if not parse it as a text log line
					if recordStr[0] == '{' && recordStr[len(recordStr)-1] == '}' {
						var record map[string]any
						err = json.Unmarshal([]byte(recordStr), &record)
						if err != nil {
							logger.Error("Error unmarshalling record:", zap.Error(err))
							// do not return, we want to continue processing the event
						} else {
							e["record"] = record
						}
					} else {
						matches := logLineRgx.FindStringSubmatch(recordStr)
						if len(matches) == 5 {
							e["record"] = map[string]any{"requestId": matches[2], "message": matches[4], "timestamp": matches[1], "level": e["level"]}
							e["level"] = strings.ToLower(matches[3])
						}
					}
				}
			}

			// decide if the handler should notify the extension that the runtime is done
			if e["type"] == "platform.runtimeDone" && !firstInvocationDone {
				notifyRuntimeDone = true
			}
		}

		// queue all the events at once to prevent locking and unlocking the mutex
		// on each event
		flusher.SafelyUseAxiomClient(ax, func(client *flusher.Axiom) {
			client.QueueEvents(events)
		})

		// inform the extension that platform.runtimeDone event has been received
		if notifyRuntimeDone {
			runtimeDone <- struct{}{}
			firstInvocationDone = true
			// close the channel since it will not be longer used
			close(runtimeDone)
		}
	}
}
