package tools

import (
	"context"
	"fmt"

	"github.com/dauren/monitoring-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterLoki(s *server.MCPServer, c *client.LokiClient) {
	s.AddTool(
		mcp.NewTool("loki_query_range",
			mcp.WithDescription("Query Loki logs over a time range using LogQL. Use this to find error logs, extract trace IDs, or see what a service was logging during an incident."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description(`LogQL stream selector + optional filter, e.g. '{app="myapp"}', '{app="api"} |= "error"', '{namespace="prod"} | json | level="error"'`),
			),
			mcp.WithString("start",
				mcp.Required(),
				mcp.Description("Start time in RFC3339 or Unix nanoseconds, e.g. '2024-01-15T10:00:00Z'"),
			),
			mcp.WithString("end",
				mcp.Required(),
				mcp.Description("End time in RFC3339 or Unix nanoseconds"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Max log lines to return (default 100, max 5000)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := req.GetString("query", "")
			start := req.GetString("start", "")
			end := req.GetString("end", "")
			if query == "" || start == "" || end == "" {
				return mcp.NewToolResultError("query, start, and end are required"), nil
			}
			limit := req.GetInt("limit", 100)
			resp, err := c.QueryRange(query, start, end, limit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("loki query failed: %v", err)), nil
			}
			return jsonResult(resp)
		},
	)

	s.AddTool(
		mcp.NewTool("loki_label_values",
			mcp.WithDescription("List all values for a Loki log label. Use this to discover available apps, namespaces, or services before writing a log query."),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Label name, e.g. 'app', 'namespace', 'service', 'job', 'level'"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			label := req.GetString("label", "")
			if label == "" {
				return mcp.NewToolResultError("label is required"), nil
			}
			resp, err := c.LabelValues(label)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get label values: %v", err)), nil
			}
			return jsonResult(resp)
		},
	)
}