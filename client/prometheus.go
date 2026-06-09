package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type PrometheusClient struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client
}

func NewPrometheusClient(baseURL, username, token string) *PrometheusClient {
	return &PrometheusClient{
		baseURL:    baseURL,
		username:   username,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type PrometheusResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType string          `json:"errorType,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func (c *PrometheusClient) Query(ctx context.Context, query, t string) (*PrometheusResponse, error) {
	params := url.Values{"query": {query}}
	if t != "" {
		params.Set("time", t)
	}
	return c.get(ctx, "/api/v1/query", params)
}

func (c *PrometheusClient) QueryRange(ctx context.Context, query, start, end, step string) (*PrometheusResponse, error) {
	return c.get(ctx, "/api/v1/query_range", url.Values{
		"query": {query},
		"start": {start},
		"end":   {end},
		"step":  {step},
	})
}

func (c *PrometheusClient) MetricNames(ctx context.Context) (*PrometheusResponse, error) {
	return c.get(ctx, "/api/v1/label/__name__/values", nil)
}

func (c *PrometheusClient) LabelValues(ctx context.Context, label string) (*PrometheusResponse, error) {
	return c.get(ctx, fmt.Sprintf("/api/v1/label/%s/values", url.PathEscape(label)), nil)
}

func (c *PrometheusClient) get(ctx context.Context, path string, params url.Values) (*PrometheusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		req.URL.RawQuery = params.Encode()
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.token)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus HTTP %d: %.500s", resp.StatusCode, body)
	}
	var result PrometheusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w (body: %.200s)", err, body)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus error (%s): %s", result.ErrorType, result.Error)
	}
	return &result, nil
}
