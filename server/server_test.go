package server

import (
	"testing"
)

func TestMessageExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:  "error messages on multiple lines",
			input: "2024-01-16T08:53:51.919Z	4b995efa-75f8-4fdc-92af-0882c79f47a1	ERROR	testing sending an error\nand this is a new line inside the error \n and a new line \n bye",
			expected: map[string]any{
				"level":   "error",
				"message": "SAME_AS_INPUT_NO_NEED_TO_DUPLICATE_INPUT_HERE",
				"record":  map[string]any{"requestId": "4b995efa-75f8-4fdc-92af-0882c79f47a1", "message": "testing sending an error\nand this is a new line inside the error \n and a new line \n bye", "timestamp": "2024-01-16T08:53:51.919Z", "level": "error"},
			},
		},
		{
			name:  "info messages",
			input: "2024-01-16T08:53:51.919Z	4b995efa-75f8-4fdc-92af-0882c79f47a2	INFO	Hello, world!",
			expected: map[string]any{
				"level":   "info",
				"message": "SAME_AS_INPUT_NO_NEED_TO_DUPLICATE_INPUT_HERE",
				"record":  map[string]any{"requestId": "4b995efa-75f8-4fdc-92af-0882c79f47a2", "message": "Hello, world!", "timestamp": "2024-01-16T08:53:51.919Z", "level": "info"},
			},
		},
		{
			name:  "warn messages",
			input: "2024-01-16T08:53:51.919Z	4b995efa-75f8-4fdc-92af-0882c79f47a3	WARN	head my warning",
			expected: map[string]any{
				"level":   "warn",
				"message": "SAME_AS_INPUT_NO_NEED_TO_DUPLICATE_INPUT_HERE",
				"record":  map[string]any{"requestId": "4b995efa-75f8-4fdc-92af-0882c79f47a3", "message": "head my warning", "timestamp": "2024-01-16T08:53:51.919Z", "level": "warn"},
			},
		},
		{
			name:  "trace messages",
			input: "2024-01-16T08:53:51.919Z	4b995efa-75f8-4fdc-92af-0882c79f47a4	TRACE	this is a trace \n with information on a new line.",
			expected: map[string]any{
				"level":   "trace",
				"message": "SAME_AS_INPUT_NO_NEED_TO_DUPLICATE_INPUT_HERE",
				"record":  map[string]any{"requestId": "4b995efa-75f8-4fdc-92af-0882c79f47a4", "message": "this is a trace \n with information on a new line.", "timestamp": "2024-01-16T08:53:51.919Z", "level": "trace"},
			},
		},
		{
			name:  "debug messages",
			input: "2024-01-16T08:53:51.919Z	4b995efa-75f8-4fdc-92af-0882c79f47a5	DEBUG	Debugging is fun!",
			expected: map[string]any{
				"level":   "debug",
				"message": "SAME_AS_INPUT_NO_NEED_TO_DUPLICATE_INPUT_HERE",
				"record":  map[string]any{"requestId": "4b995efa-75f8-4fdc-92af-0882c79f47a5", "message": "Debugging is fun!", "timestamp": "2024-01-16T08:53:51.919Z", "level": "debug"},
			},
		},
		{
			name:  "testing json messages",
			input: `{"timestamp":"2024-01-08T16:48:45.316Z","level":"INFO","requestId":"de126cf0-6124-426c-818a-174983fbfc4b","message":"foo != bar"}`,
			expected: map[string]any{
				"level":   "info",
				"message": "SAME_AS_INPUT_NO_NEED_TO_DUPLICATE_INPUT_HERE",
				"record":  map[string]any{"requestId": "de126cf0-6124-426c-818a-174983fbfc4b", "message": "foo != bar", "timestamp": "2024-01-08T16:48:45.316Z", "level": "info"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			e := make(map[string]any)
			e["record"] = testCase.input
			extractEventMessage(e)
			if e["level"] != testCase.expected["level"] {
				t.Errorf("Expected level to be %s, got %s", testCase.expected["level"], e["level"])
			}
			if e["message"] != testCase.input { // the message field should contain the original input
				t.Errorf("Expected message to be %s, got %s", testCase.input, e["message"])
			}

			expectedRecord := testCase.expected["record"].(map[string]any)
			outputRecord := e["record"].(map[string]any)

			if outputRecord["timestamp"] != expectedRecord["timestamp"] {
				t.Errorf("Expected timestamp to be %s, got %s", testCase.expected["timestamp"], e["timestamp"])
			}
			if outputRecord["level"] != expectedRecord["level"] {
				t.Errorf("Expected record.level to be %s, got %s", expectedRecord["level"], outputRecord["level"])
			}
			if outputRecord["requestId"] != expectedRecord["requestId"] {
				t.Errorf("Expected record.requestId to be %s, got %s", expectedRecord["requestId"], outputRecord["requestId"])
			}
			if outputRecord["message"] != expectedRecord["message"] {
				t.Errorf("Expected record.message to be %s, got %s", expectedRecord["message"], outputRecord["message"])
			}
		})
	}
}
