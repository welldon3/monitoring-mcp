package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/dauren/monitoring-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterTempo(s *server.MCPServer, c *client.TempoClient) {
	s.AddTool(
		mcp.NewTool("tempo_search",
			mcp.WithDescription("Search for traces in Tempo by service, operation, or attributes. Returns trace IDs with root service/operation and duration. Follow up with tempo_get_trace to inspect spans."),
			mcp.WithString("tags",
				mcp.Description("Space-separated key=value tag filters, e.g. 'service.name=myapp http.status_code=500 http.method=POST'"),
			),
			mcp.WithString("min_duration",
				mcp.Description("Minimum trace duration, e.g. '100ms', '1s', '1.5s'"),
			),
			mcp.WithString("max_duration",
				mcp.Description("Maximum trace duration, e.g. '500ms', '2s'"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Max traces to return (default 20)"),
			),
			mcp.WithString("start",
				mcp.Description("Start time as Unix timestamp in seconds"),
			),
			mcp.WithString("end",
				mcp.Description("End time as Unix timestamp in seconds"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.Search(
				req.GetString("tags", ""),
				req.GetString("min_duration", ""),
				req.GetString("max_duration", ""),
				req.GetInt("limit", 20),
				req.GetString("start", ""),
				req.GetString("end", ""),
			)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("tempo search failed: %v", err)), nil
			}
			return jsonResult(resp)
		},
	)

	s.AddTool(
		mcp.NewTool("tempo_get_trace",
			mcp.WithDescription("Retrieve a full trace by ID, including all spans, timings, and service relationships. Use this after tempo_search or when you have a trace_id from a log line."),
			mcp.WithString("trace_id",
				mcp.Required(),
				mcp.Description("Trace ID (hex string) from tempo_search or extracted from a log line"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			traceID := strings.TrimSpace(req.GetString("trace_id", ""))
			if traceID == "" {
				return mcp.NewToolResultError("trace_id is required"), nil
			}
			data, err := c.GetTrace(traceID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get trace: %v", err)), nil
			}
			return jsonBytesResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool("tempo_search_tags",
			mcp.WithDescription("List all searchable tag names in Tempo. Use this to discover what attributes are available for filtering in tempo_search."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			tags, err := c.SearchTags()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to list tags: %v", err)), nil
			}
			return jsonResult(tags)
		},
	)

	s.AddTool(
		mcp.NewTool("tempo_tag_values",
			mcp.WithDescription("List all values for a specific Tempo tag. Use this to find the exact service name or operation name to use as a filter in tempo_search."),
			mcp.WithString("tag",
				mcp.Required(),
				mcp.Description("Tag name, e.g. 'service.name', 'http.method', 'http.status_code'"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			tag := req.GetString("tag", "")
			if tag == "" {
				return mcp.NewToolResultError("tag is required"), nil
			}
			values, err := c.SearchTagValues(tag)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get tag values: %v", err)), nil
			}
			return jsonResult(values)
		},
	)
}