package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 4 * time.Minute

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	url := c.config.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
	return req, nil
}

func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (<-chan string, <-chan error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		tokenCh := make(chan string)
		errCh := make(chan error, 1)
		close(tokenCh)
		errCh <- fmt.Errorf("marshal request: %w", err)
		close(errCh)
		return tokenCh, errCh
	}

	httpReq, err := c.newRequest("POST", "/chat/completions", bytes.NewReader(body))
	if err != nil {
		tokenCh := make(chan string)
		errCh := make(chan error, 1)
		close(tokenCh)
		errCh <- fmt.Errorf("create request: %w", err)
		close(errCh)
		return tokenCh, errCh
	}
	httpReq = httpReq.WithContext(ctx)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		tokenCh := make(chan string)
		errCh := make(chan error, 1)
		close(tokenCh)
		errCh <- fmt.Errorf("request failed: %w", err)
		close(errCh)
		return tokenCh, errCh
	}

	return parseSSEStream(resp)
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (string, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := c.newRequest("POST", "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq = httpReq.WithContext(ctx)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}

	return joinStream(resp)
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := c.newRequest("GET", "/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req = req.WithContext(ctx)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list models returned %d: %s", resp.StatusCode, string(body))
	}

	var ml ModelList
	if err := json.NewDecoder(resp.Body).Decode(&ml); err != nil {
		return nil, fmt.Errorf("parse model list: %w", err)
	}

	models := make([]string, 0, len(ml.Data))
	for _, m := range ml.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func (c *Client) HealthCheck(ctx context.Context) bool {
	req, err := c.newRequest("GET", "/models", nil)
	if err != nil {
		return false
	}
	req = req.WithContext(ctx)

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
