package register

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// impl is a minimal MCP Implementation descriptor used in SDK calls.
var impl = &mcp.Implementation{Name: "test", Version: "v0.0.1"}

type testInput struct {
	Query string `json:"query"`
}

type testOutput struct {
	Answer string `json:"answer"`
}

func TestAdapt_SuccessPath(t *testing.T) {
	h := handlers.Handler[testInput, testOutput](func(_ context.Context, in testInput) (testOutput, error) {
		return testOutput{Answer: "hello " + in.Query}, nil
	})

	sdkHandler := adapt(h)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	in := testInput{Query: "world"}

	result, out, err := sdkHandler(ctx, req, in)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	// Verify structured output
	require.Equal(t, testOutput{Answer: "hello world"}, out)

	// Verify Content has exactly one TextContent
	require.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent")

	// Verify TextContent is valid JSON of Out
	var decoded testOutput
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &decoded))
	require.Equal(t, out, decoded)
}

func TestAdapt_ErrorPath(t *testing.T) {
	wantErr := errors.New("something went wrong")
	h := handlers.Handler[testInput, testOutput](func(_ context.Context, _ testInput) (testOutput, error) {
		return testOutput{}, wantErr
	})

	sdkHandler := adapt(h)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	in := testInput{Query: "boom"}

	result, out, err := sdkHandler(ctx, req, in)

	// adapt returns (nil, zero, err) so the SDK can convert it to
	// CallToolResult{IsError: true, Content: [TextContent{err.Error()}]}.
	// The SDK contract is tested end-to-end in TestAdapt_ErrorPath_ViaSDK.
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, result)
	require.Equal(t, testOutput{}, out)
}

// TestAdapt_ErrorPath_ViaSDK verifies the full SDK error-path contract:
// when adapt returns (nil, zero, err), the SDK wraps the error into
// CallToolResult{IsError: true} and returns nil transport error to the caller.
// This matches the comment in the SDK test: "Counter-intuitively, when a tool
// fails, we don't expect an RPC error for call tool: instead, the failure is
// embedded in the result."
func TestAdapt_ErrorPath_ViaSDK(t *testing.T) {
	const toolName = "failing_tool"
	wantMsg := "something went wrong via sdk"
	wantErr := errors.New(wantMsg)

	h := handlers.Handler[testInput, testOutput](func(_ context.Context, _ testInput) (testOutput, error) {
		return testOutput{}, wantErr
	})

	// Build a real MCP server with the adapted handler registered.
	srv := mcp.NewServer(impl, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: toolName}, adapt(h))

	// Wire server and client via an in-memory transport pair.
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()

	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(impl, nil)
	cs, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)

	// Call the failing tool through the client session.
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: map[string]any{"query": "boom"},
	})

	// The SDK must NOT return a transport/RPC error — the error is embedded.
	require.NoError(t, err, "tool errors must be embedded in the result, not returned as transport errors")
	require.NotNil(t, result)
	require.True(t, result.IsError, "result.IsError must be true when handler returned an error")

	// Content must contain exactly one TextContent with the error message.
	require.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent in error result")
	require.Equal(t, wantMsg, tc.Text)
}
