package logsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type BufferingCfg struct {
	MaxItems  uint32 `json:"maxItems"`
	MaxBytes  uint32 `json:"maxBytes"`
	TimeoutMS uint32 `json:"timeoutMs"`
}

// URI is used to set the endpoint where the logs will be sent to
type URI string

// HttpMethod represents the HTTP method used to receive logs from Logs API
type HttpMethod string

const (
	//HttpPost is to receive logs through POST.
	HttpPost HttpMethod = "POST"
	//HttpPUT is to receive logs through PUT.
	HttpPut HttpMethod = "PUT"
)

// HttpProtocol is used to specify the protocol when subscribing to Logs API for HTTP
type HttpProtocol string

const (
	HttpProto HttpProtocol = "HTTP"
)

// HttpEncoding denotes what the content is encoded in
type HttpEncoding string

const (
	JSON HttpEncoding = "JSON"
)

type Destination struct {
	Protocol   HttpProtocol `json:"protocol"`
	URI        URI          `json:"URI"`
	HttpMethod HttpMethod   `json:"method"`
	Encoding   HttpEncoding `json:"encoding"`
}

type SubscribeRequest struct {
	SchemaVersion string       `json:"schemaVersion"`
	EventTypes    []string     `json:"types"`
	BufferingCfg  BufferingCfg `json:"buffering"`
	Destination   Destination  `json:"destination"`
}

// SubscribeResponse is the response body that is received from Logs API on subscribe
type SubscribeResponse struct {
	body string
}

const (
	lambdaAgentIdentifierHeaderKey = "Lambda-Extension-Identifier"
	SchemaVersion20210318          = "2021-03-18"
	SchemaVersionLatest            = SchemaVersion20210318
)

func New(LogsAPI string) *Client {
	return &Client{
		baseURL:    fmt.Sprintf("http://%s/2020-08-15", LogsAPI),
		httpClient: &http.Client{},
	}
}

func (lc *Client) Subscribe(ctx context.Context, types []string, bufferingCfg BufferingCfg, destination Destination, extensionID string) (*SubscribeResponse, error) {
	subscribeEndpoint := lc.baseURL + "/logs"

	subReq, err := json.Marshal(
		&SubscribeRequest{
			SchemaVersion: SchemaVersionLatest,
			EventTypes:    types,
			BufferingCfg:  bufferingCfg,
			Destination:   destination,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("Marshaling subscribeRequest Failed")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", subscribeEndpoint, bytes.NewBuffer(subReq))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set(lambdaAgentIdentifierHeaderKey, extensionID)
	httpRes, err := lc.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	defer httpRes.Body.Close()
	body, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}

	if httpRes.StatusCode != 200 {
		return nil, fmt.Errorf("Subscription Request Failed with Status %s", httpRes.Status)
	}
	return &SubscribeResponse{
		body: string(body),
	}, nil
}
