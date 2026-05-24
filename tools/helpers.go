package tools

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

func jsonResult(v interface{}) (*mcp.CallToolResult, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to encode response: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// jsonBytesResult pretty-prints raw JSON bytes.
func jsonBytesResult(data []byte) (*mcp.CallToolResult, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return mcp.NewToolResultText(string(data)), nil
	}
	return jsonResult(v)
}