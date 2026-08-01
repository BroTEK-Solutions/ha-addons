package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type supervisorClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type supervisorEnvelope struct {
	Result  string          `json:"result"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newSupervisorClient(baseURL, token string, client *http.Client) *supervisorClient {
	return &supervisorClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    client,
	}
}

func (c *supervisorClient) Restart(ctx context.Context) error {
	return c.call(ctx, http.MethodPost, "/addons/self/restart", map[string]any{}, nil)
}

func (c *supervisorClient) call(ctx context.Context, method, path string, payload any, data any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode Supervisor request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create Supervisor request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call Supervisor: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Supervisor returned %s: %s", resp.Status, strings.TrimSpace(string(limited)))
	}
	var envelope supervisorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode Supervisor response: %w", err)
	}
	if envelope.Result != "ok" && envelope.Result != "" {
		if envelope.Message == "" {
			envelope.Message = "unknown Supervisor error"
		}
		return errors.New(envelope.Message)
	}
	if data != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, data); err != nil {
			return fmt.Errorf("decode Supervisor data: %w", err)
		}
	}
	return nil
}
