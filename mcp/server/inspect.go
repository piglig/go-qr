package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/piglig/go-qr/mcp/internal/inspect"
)

// InspectInput is QR Code text to explain.
type InspectInput struct {
	Text string `json:"text" jsonschema:"the decoded content of a QR Code"`
	GS1  bool   `json:"gs1,omitempty" jsonschema:"the code was a GS1 symbol"`
}

func inspectText(_ context.Context, _ *mcp.CallToolRequest, in InspectInput) (*mcp.CallToolResult, inspect.Report, error) {
	return nil, inspect.Text(in.Text, in.GS1), nil
}
