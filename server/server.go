package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"os"
	"strconv"
	"strings"

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

var logLineRgx = regexp.MustCompile(`^([0-9.:TZ-]{20,})\s+([0-9a-f-]{36})\s+(ERROR|INFO|WARN|DEBUG|TRACE)\s+(?s:(.*))`)

// Repeated event field/value literals, extracted to satisfy the goconst linter.
const (
	eventTypeFunction = "function"
	fieldType         = "type"
	fieldRecord       = "record"
	fieldRequestID    = "requestId"
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
		requestID := ""

		for _, e := range events {
			if record, ok := e[fieldRecord].(map[string]any); ok {
				if id, ok := stringField(record, fieldRequestID); ok {
					requestID = id
				}
			}

			// attach the lambda information to the event
			e["lambda"] = lambdaMetaInfo
			e["axiom"] = axiomMetaInfo
			// replace the time field with axiom's _time
			e["_time"], e["time"] = e["time"], nil

			if e[fieldType] == eventTypeFunction {
				requestID = extractEventMessage(e, requestID)
			}

			// decide if the handler should notify the extension that the runtime is done
			if e[fieldType] == "platform.runtimeDone" && !firstInvocationDone {
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

// extractEventMessage normalizes Lambda function logs while preserving the raw
// record value in message. String records may contain JSON even when Telemetry
// API delivers them as text.
func extractEventMessage(e map[string]any, requestID string) string {
	recordValue, ok := e[fieldRecord]
	if !ok {
		e["message"] = ""
		e[fieldRecord] = map[string]string{fieldRequestID: requestID}
		return requestID
	}

	e["message"] = recordValue

	switch record := recordValue.(type) {
	case map[string]any:
		if id, ok := stringField(record, fieldRequestID); ok {
			requestID = id
		}
		if msg, ok := stringField(record, "message"); ok {
			e["message"] = msg
		}
		return requestID
	case string:
		return extractStringRecord(e, record, requestID)
	default:
		e[fieldRecord] = map[string]string{fieldRequestID: requestID}
		return requestID
	}
}

func extractStringRecord(e map[string]any, recordStr string, requestID string) string {
	trimmedRecord := strings.TrimSpace(recordStr)
	if trimmedRecord == "" {
		e[fieldRecord] = map[string]string{fieldRequestID: requestID}
		return requestID
	}

	if strings.HasPrefix(trimmedRecord, "{") && strings.HasSuffix(trimmedRecord, "}") {
		var record map[string]any
		if err := json.Unmarshal([]byte(trimmedRecord), &record); err != nil {
			logger.Error("Error unmarshalling record:", zap.Error(err))
		} else {
			if level, ok := stringField(record, "level"); ok {
				record["level"] = strings.ToLower(level)
				e["level"] = record["level"]
			}
			if id, ok := stringField(record, fieldRequestID); ok {
				requestID = id
			}
			e[fieldRecord] = record
			return requestID
		}
	}

	matches := logLineRgx.FindStringSubmatch(trimmedRecord)
	if len(matches) == 5 {
		e["level"] = strings.ToLower(matches[3])
		e[fieldRecord] = map[string]any{
			fieldRequestID: matches[2],
			"message":      matches[4],
			"timestamp":    matches[1],
			"level":        e["level"],
		}
		return matches[2]
	}

	e[fieldRecord] = map[string]string{fieldRequestID: requestID}
	return requestID
}

func stringField(record map[string]any, key string) (string, bool) {
	value, ok := record[key].(string)
	return value, ok
}
