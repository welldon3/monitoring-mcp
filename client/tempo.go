package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type TempoClient struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client
}

func NewTempoClient(baseURL, username, token string) *TempoClient {
	return &TempoClient{
		baseURL:    baseURL,
		username:   username,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type TraceSearchResponse struct {
	Traces []TraceSearchHit `json:"traces"`
}

type TraceSearchHit struct {
	TraceID           string            `json:"traceID"`
	RootServiceName   string            `json:"rootServiceName"`
	RootTraceName     string            `json:"rootTraceName"`
	StartTimeUnixNano string            `json:"startTimeUnixNano"`
	DurationMs        int64             `json:"durationMs"`
	SpanSets          []SpanSet         `json:"spanSets,omitempty"`
}

type SpanSet struct {
	Spans   []Span `json:"spans"`
	Matched int    `json:"matched"`
}

type Span struct {
	SpanID            string            `json:"spanID"`
	StartTimeUnixNano string            `json:"startTimeUnixNano"`
	DurationNanos     string            `json:"durationNanos"`
	Attributes        map[string]string `json:"attributes,omitempty"`
}

func (c *TempoClient) Search(tags, minDuration, maxDuration string, limit int, start, end string) (*TraceSearchResponse, error) {
	params := url.Values{}
	if tags != "" {
		params.Set("tags", tags)
	}
	if minDuration != "" {
		params.Set("minDuration", minDuration)
	}
	if maxDuration != "" {
		params.Set("maxDuration", maxDuration)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if start != "" {
		params.Set("start", start)
	}
	if end != "" {
		params.Set("end", end)
	}
	body, err := c.getRaw("/api/search", params)
	if err != nil {
		return nil, err
	}
	var result TraceSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *TempoClient) GetTrace(traceID string) (json.RawMessage, error) {
	body, err := c.getRaw(fmt.Sprintf("/api/traces/%s", traceID), nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

func (c *TempoClient) SearchTags() ([]string, error) {
	body, err := c.getRaw("/api/search/tags", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		TagNames []string `json:"tagNames"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.TagNames, nil
}

func (c *TempoClient) SearchTagValues(tag string) ([]string, error) {
	body, err := c.getRaw(fmt.Sprintf("/api/search/tag/%s/values", url.PathEscape(tag)), nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		TagValues []string `json:"tagValues"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.TagValues, nil
}

func (c *TempoClient) getRaw(path string, params url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
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
	req.Header.Set("Accept", "application/json")
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
		return nil, fmt.Errorf("tempo HTTP %d: %.500s", resp.StatusCode, body)
	}
	return body, nil
}
