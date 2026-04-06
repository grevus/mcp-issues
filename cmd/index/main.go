package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/grevus/mcp-jira/internal/config"
	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/grevus/mcp-jira/internal/rag/embed"
	ragindex "github.com/grevus/mcp-jira/internal/rag/index"
	"github.com/grevus/mcp-jira/internal/rag/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch os.Args[1] {
	case "migrate":
		runMigrate(ctx)
	case "index":
		runIndex(ctx, os.Args[2:])
	default:
		log.Printf("unknown subcommand %q", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	log.Println("usage: mcp-jira-index <subcommand> [flags]")
	log.Println("  migrate                  run database migrations")
	log.Println("  index --project=KEY      reindex a Jira project")
}

func runIndex(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	projectKey := fs.String("project", "", "Jira project key to reindex (required)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("index: parse flags: %v", err)
	}
	if *projectKey == "" {
		log.Println("index: --project=KEY is required")
		fs.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load(config.ModeIndex)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	jc := jira.NewHTTPClient(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, nil)

	var emb embed.Embedder
	switch cfg.RAGEmbedder {
	case "openai":
		emb = embed.NewOpenAIEmbedder(cfg.OpenAIAPIKey, nil)
	default: // "voyage"
		emb = embed.NewVoyageEmbedder(cfg.VoyageAPIKey, nil)
	}

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	indexer := ragindex.New(jc, emb, st)

	start := time.Now()
	n, err := indexer.Reindex(ctx, *projectKey)
	if err != nil {
		log.Fatalf("reindex: %v", err)
	}
	log.Printf("indexed %d docs in %s", n, time.Since(start))
}

func runMigrate(ctx context.Context) {
	cfg, err := config.Load(config.ModeIndex)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Printf("migrations applied")
}
