package tools

import (
	"context"
	"fmt"

	"github.com/dauren/monitoring-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterPrometheus(s *server.MCPServer, c *client.PrometheusClient) {
	s.AddTool(
		mcp.NewTool("prometheus_query",
			mcp.WithDescription("Execute a PromQL instant query. Use this to check current metric values, alert conditions, or spot-check a specific metric at a point in time."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description(`PromQL expression, e.g. 'up', 'rate(http_requests_total[5m])', 'sum by (job) (rate(errors_total[1m]))'`),
			),
			mcp.WithString("time",
				mcp.Description("Evaluation timestamp in RFC3339 or Unix seconds. Omit for now."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := req.GetString("query", "")
			if query == "" {
				return mcp.NewToolResultError("query is required"), nil
			}
			resp, err := c.Query(query, req.GetString("time", ""))
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
			}
			return jsonResult(resp)
		},
	)

	s.AddTool(
		mcp.NewTool("prometheus_query_range",
			mcp.WithDescription("Execute a PromQL range query to analyze a metric over time. Use this to spot trends, compare before/after an incident, or graph error rates over a window."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("PromQL expression"),
			),
			mcp.WithString("start",
				mcp.Required(),
				mcp.Description("Start time in RFC3339 or Unix seconds, e.g. '2024-01-15T10:00:00Z'"),
			),
			mcp.WithString("end",
				mcp.Required(),
				mcp.Description("End time in RFC3339 or Unix seconds"),
			),
			mcp.WithString("step",
				mcp.Required(),
				mcp.Description("Resolution step, e.g. '15s', '1m', '5m', '1h'"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := req.GetString("query", "")
			start := req.GetString("start", "")
			end := req.GetString("end", "")
			step := req.GetString("step", "")
			if query == "" || start == "" || end == "" || step == "" {
				return mcp.NewToolResultError("query, start, end, and step are required"), nil
			}
			resp, err := c.QueryRange(query, start, end, step)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("range query failed: %v", err)), nil
			}
			return jsonResult(resp)
		},
	)

	s.AddTool(
		mcp.NewTool("prometheus_metric_names",
			mcp.WithDescription("List all metric names available in Prometheus. Use this first to discover what metrics exist before writing queries."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			resp, err := c.MetricNames()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to list metric names: %v", err)), nil
			}
			return jsonResult(resp)
		},
	)

	s.AddTool(
		mcp.NewTool("prometheus_label_values",
			mcp.WithDescription("List all values for a Prometheus label. Useful for discovering job names, instances, namespaces, or services before filtering a query."),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Label name, e.g. 'job', 'instance', 'namespace', 'service'"),
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