package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/grevus/mcp-jira/internal/config"
	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/grevus/mcp-jira/internal/rag/embed"
	"github.com/grevus/mcp-jira/internal/rag/retriever"
	"github.com/grevus/mcp-jira/internal/rag/store"
	"github.com/grevus/mcp-jira/internal/register"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

	log.Println("mcp-jira: dependencies wired, transport not started yet")
	_ = srv
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
