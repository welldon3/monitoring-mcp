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

type LokiClient struct {
	baseURL    string
	username   string
	token      string
	orgID      string
	httpClient *http.Client
}

func NewLokiClient(baseURL, username, token, orgID string) *LokiClient {
	return &LokiClient{
		baseURL:    baseURL,
		username:   username,
		token:      token,
		orgID:      orgID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type LokiResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

func (c *LokiClient) QueryRange(ctx context.Context, query, start, end string, limit int) (*LokiResponse, error) {
	return c.get(ctx, "/loki/api/v1/query_range", url.Values{
		"query": {query},
		"start": {start},
		"end":   {end},
		"limit": {fmt.Sprintf("%d", limit)},
	})
}

func (c *LokiClient) Labels(ctx context.Context) (*LokiResponse, error) {
	return c.get(ctx, "/loki/api/v1/labels", nil)
}

func (c *LokiClient) LabelValues(ctx context.Context, label string) (*LokiResponse, error) {
	return c.get(ctx, fmt.Sprintf("/loki/api/v1/label/%s/values", url.PathEscape(label)), nil)
}

func (c *LokiClient) get(ctx context.Context, path string, params url.Values) (*LokiResponse, error) {
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
	if c.orgID != "" {
		req.Header.Set("X-Scope-OrgID", c.orgID)
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
		return nil, fmt.Errorf("loki HTTP %d: %.500s", resp.StatusCode, body)
	}
	var result LokiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w (body: %.200s)", err, body)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("loki error: %.500s", body)
	}
	return &result, nil
}
