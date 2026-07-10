package server

import "testing"

func TestExtractEventMessageParsesJSONRecordString(t *testing.T) {
	rawRecord := `{"level":"INFO","env":"test_creds","app":"aall","requestId":"eb506e5d-e205-4747-870b-b85a48fc2f19","gatewayRequestId":"Root=1-69de4a4b-0d6b7a832500a9a9340fabb1","method":"GET","path":"/brands/cgra8memi4ohud77svp0/overview/2026-03-15/2026-04-14","handler_time_ms":80,"time":"2026-04-14T14:08:11Z","caller":"/home/runner/work/unify/unify/internal/middleware/timing.go:32","message":"request completed"}` + "\n"
	event := map[string]any{
		fieldType:   eventTypeFunction,
		fieldRecord: rawRecord,
	}

	requestID := extractEventMessage(event, "platform-request-id")

	if requestID != "eb506e5d-e205-4747-870b-b85a48fc2f19" {
		t.Fatalf("expected requestID from parsed record, got %q", requestID)
	}
	if event["message"] != rawRecord {
		t.Fatalf("expected message to preserve raw record")
	}
	if event["level"] != "info" {
		t.Fatalf("expected top-level level to be normalized, got %v", event["level"])
	}

	record, ok := event[fieldRecord].(map[string]any)
	if !ok {
		t.Fatalf("expected parsed record map, got %T", event[fieldRecord])
	}

	assertEqual(t, record["app"], "aall")
	assertEqual(t, record["level"], "info")
	assertEqual(t, record["message"], "request completed")
	assertEqual(t, record["path"], "/brands/cgra8memi4ohud77svp0/overview/2026-03-15/2026-04-14")
	assertEqual(t, record[fieldRequestID], "eb506e5d-e205-4747-870b-b85a48fc2f19")
}

func TestExtractEventMessagePreservesStructuredTelemetryRecord(t *testing.T) {
	inputRecord := map[string]any{
		fieldRequestID: "structured-request-id",
		"message":      "request completed",
		"level":        "info",
	}
	event := map[string]any{
		fieldType:   eventTypeFunction,
		fieldRecord: inputRecord,
	}

	requestID := extractEventMessage(event, "platform-request-id")

	if requestID != "structured-request-id" {
		t.Fatalf("expected requestID from structured record, got %q", requestID)
	}
	if event["message"] != "request completed" {
		t.Fatalf("expected message from structured record, got %v", event["message"])
	}
	record, ok := event[fieldRecord].(map[string]any)
	if !ok {
		t.Fatalf("expected structured record map, got %T", event[fieldRecord])
	}
	assertEqual(t, record[fieldRequestID], "structured-request-id")
	assertEqual(t, record["message"], "request completed")
	assertEqual(t, record["level"], "info")
}

func TestExtractEventMessageFallsBackForPlainStringRecord(t *testing.T) {
	event := map[string]any{
		fieldType:   eventTypeFunction,
		fieldRecord: "plain log line",
	}

	requestID := extractEventMessage(event, "platform-request-id")

	if requestID != "platform-request-id" {
		t.Fatalf("expected existing requestID to be preserved, got %q", requestID)
	}
	if event["message"] != "plain log line" {
		t.Fatalf("expected message to preserve plain log line, got %v", event["message"])
	}

	record, ok := event[fieldRecord].(map[string]string)
	if !ok {
		t.Fatalf("expected fallback record map, got %T", event[fieldRecord])
	}
	assertEqual(t, record[fieldRequestID], "platform-request-id")
}

func TestExtractEventMessageParsesAWSLogLine(t *testing.T) {
	rawRecord := "2024-01-16T08:53:51.919Z\t4b995efa-75f8-4fdc-92af-0882c79f47a1\tERROR\ttesting sending an error\nand this is a new line inside the error"
	event := map[string]any{
		fieldType:   eventTypeFunction,
		fieldRecord: rawRecord,
	}

	requestID := extractEventMessage(event, "")

	if requestID != "4b995efa-75f8-4fdc-92af-0882c79f47a1" {
		t.Fatalf("expected requestID from parsed log line, got %q", requestID)
	}
	if event["message"] != rawRecord {
		t.Fatalf("expected message to preserve raw log line")
	}
	if event["level"] != "error" {
		t.Fatalf("expected parsed level, got %v", event["level"])
	}

	record, ok := event[fieldRecord].(map[string]any)
	if !ok {
		t.Fatalf("expected parsed log line record map, got %T", event[fieldRecord])
	}
	assertEqual(t, record[fieldRequestID], "4b995efa-75f8-4fdc-92af-0882c79f47a1")
	assertEqual(t, record["level"], "error")
	assertEqual(t, record["message"], "testing sending an error\nand this is a new line inside the error")
	assertEqual(t, record["timestamp"], "2024-01-16T08:53:51.919Z")
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
