package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseURL     string
	httpClient  *http.Client
	ExtensionID string
}

type RegisterResponse struct {
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	Handler         string `json:"handler"`
}

type Tracing struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type EventType string

type NextEventResponse struct {
	EventType          EventType `json:"eventType"`
	DeadlineMs         int64     `json:"deadlineMs"`
	RequestID          string    `json:"requestId"`
	InvokedFunctionArn string    `json:"invokedFunctionArn"`
	Tracing            Tracing   `json:"tracing"`
}

const (
	extensionNameHeader       = "Lambda-Extension-Name"
	extensionIdentifierHeader = "Lambda-Extension-Identifier"
)

func New(telemetryAPI string) *Client {
	return &Client{
		baseURL:    fmt.Sprintf("http://%s/2020-01-01/extension", telemetryAPI),
		httpClient: &http.Client{},
	}
}

func (c *Client) Register(ctx context.Context, extensionName string) (*RegisterResponse, error) {
	registerEndpoint := c.baseURL + "/register"

	reqBody, err := json.Marshal(map[string]any{
		"events": []string{"INVOKE", "SHUTDOWN"},
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", registerEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set(extensionNameHeader, extensionName)

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpRes.StatusCode != 200 {
		return nil, fmt.Errorf("Register Request Failed with status %s", httpRes.Status)
	}

	defer httpRes.Body.Close()
	body, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}

	RegRes := RegisterResponse{}
	err = json.Unmarshal(body, &RegRes)
	if err != nil {
		return nil, err
	}

	c.ExtensionID = httpRes.Header.Get(extensionIdentifierHeader)
	return &RegRes, nil
}

func (c *Client) NextEvent(ctx context.Context, extensionName string) (*NextEventResponse, error) {
	nextEventEndpoint := c.baseURL + "/event/next"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", nextEventEndpoint, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set(extensionNameHeader, extensionName)
	httpReq.Header.Set(extensionIdentifierHeader, c.ExtensionID)

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpRes.Body.Close()
	if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Next Event Failed with status %s", httpRes.Status)
	}
	body, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}
	res := NextEventResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}
