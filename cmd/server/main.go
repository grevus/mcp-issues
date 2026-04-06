package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/grevus/mcp-jira/internal/auth"
	"github.com/grevus/mcp-jira/internal/config"
	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/grevus/mcp-jira/internal/rag/embed"
	"github.com/grevus/mcp-jira/internal/rag/retriever"
	"github.com/grevus/mcp-jira/internal/rag/store"
	"github.com/grevus/mcp-jira/internal/register"
)

func main() {
	transport := flag.String("transport", "stdio", "Transport to use: stdio or http")
	flag.Parse()

	var mode config.Mode
	switch *transport {
	case "stdio":
		mode = config.ModeStdio
	case "http":
		mode = config.ModeHTTP
	default:
		log.Fatalf("unknown transport %q: must be stdio or http", *transport)
	}

	cfg, err := config.Load(mode)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Jira HTTP client.
	jc := jira.NewHTTPClient(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, nil)

	// Embedder: switch by cfg.RAGEmbedder.
	var emb embed.Embedder
	switch cfg.RAGEmbedder {
	case "openai":
		emb = embed.NewOpenAIEmbedder(cfg.OpenAIAPIKey, nil)
	default: // "voyage"
		emb = embed.NewVoyageEmbedder(cfg.VoyageAPIKey, nil)
	}

	// PgvectorStore.
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	// Retriever.
	ret := retriever.New(emb, st)

	// retrieverAdapter bridges retriever.Retriever (returns []store.Hit)
	// to handlers.KnowledgeRetriever (expects []handlers.Hit).
	retAdapter := &retrieverAdapter{r: ret}

	// MCP server.
	srv := mcp.NewServer(&mcp.Implementation{Name: "mcp-jira", Version: "0.1.0"}, nil)
	register.Register(srv, jc, retAdapter)

	switch mode {
	case config.ModeStdio:
		// stdio: never write to stdout — it's reserved for JSON-RPC.
		log.SetOutput(os.Stderr)
		log.Println("mcp-jira: starting stdio transport")
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Fatalf("stdio transport: %v", err)
		}

	case config.ModeHTTP:
		e := echo.New()
		e.Use(echo.WrapMiddleware(auth.Middleware(cfg.MCPAPIKey)))

		mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		e.Any("/mcp", echo.WrapHandler(mcpHandler))

		log.Printf("mcp-jira: starting HTTP transport on %s", cfg.MCPAddr)
		if err := e.Start(cfg.MCPAddr); err != nil {
			log.Fatalf("http transport: %v", err)
		}
	}
}

// retrieverAdapter converts retriever.Retriever output ([]store.Hit) to
// the []handlers.Hit shape expected by handlers.KnowledgeRetriever.
type retrieverAdapter struct {
	r *retriever.Retriever
}

func (a *retrieverAdapter) Search(ctx context.Context, projectKey, query string, topK int) ([]handlers.Hit, error) {
	hits, err := a.r.Search(ctx, projectKey, query, topK)
	if err != nil {
		return nil, err
	}
	out := make([]handlers.Hit, len(hits))
	for i, h := range hits {
		out[i] = handlers.Hit{
			IssueKey: h.IssueKey,
			Summary:  h.Summary,
			Status:   h.Status,
			Score:    h.Score,
			Excerpt:  h.Excerpt,
		}
	}
	return out, nil
}
