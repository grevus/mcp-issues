package register

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grevus/mcp-jira/internal/auth"
	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// adapt wraps a handlers.Handler[In, Out] into the SDK-compatible handler
// signature expected by mcp.AddTool.
//
// Success path: marshals Out as JSON text content and returns it as
// StructuredContent so the MCP client receives both human-readable text and
// typed structured output.
//
// Error path: delegates to the SDK convention — returns (*CallToolResult, Out,
// error) with IsError=true embedded by the SDK wrapper automatically.
func adapt[In, Out any](h handlers.Handler[In, Out]) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		out, err := h(ctx, in)
		if err != nil {
			var zero Out
			return nil, zero, err
		}

		text, marshalErr := jsonString(out)
		if marshalErr != nil {
			var zero Out
			return nil, zero, fmt.Errorf("adapt: marshal output: %w", marshalErr)
		}

		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: text},
			},
		}
		return result, out, nil
	}
}

// jsonString marshals v to a JSON string.
func jsonString(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// adaptTenant wraps a per-tenant handler factory into an SDK-compatible handler.
// It resolves the tenant from the Registry using the key name stored in context
// by MultiKeyMiddleware, then delegates to adapt.
func adaptTenant[In, Out any](
	reg *tenant.Registry,
	factory func(t *tenant.Tenant) handlers.Handler[In, Out],
) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		keyName := auth.KeyNameFromContext(ctx)
		t, err := reg.Resolve(keyName)
		if err != nil {
			var zero Out
			return nil, zero, err
		}
		h := factory(t)
		return adapt(h)(ctx, req, in)
	}
}
