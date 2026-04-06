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

	// adapt propagates the error so the SDK can wrap it in IsError=true result
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, result)
	require.Equal(t, testOutput{}, out)
}
