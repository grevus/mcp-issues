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
	"github.com/grevus/mcp-jira/internal/knowledge/embed"
	kindex "github.com/grevus/mcp-jira/internal/knowledge/index"
	kpg "github.com/grevus/mcp-jira/internal/knowledge/pgvector"
	"github.com/grevus/mcp-jira/internal/tenant"
	jiratracker "github.com/grevus/mcp-jira/internal/tracker/jira"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	config.LoadDotEnv(".env")

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
	log.Println("  migrate                              run database migrations")
	log.Println("  index --project=KEY                  reindex a Jira project (legacy env mode)")
	log.Println("  index --project=KEY --tenant=NAME --keys-file=PATH  reindex for a specific tenant")
}

func runIndex(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	projectKey := fs.String("project", "", "Jira project key to reindex (required)")
	tenantName := fs.String("tenant", "", "Tenant name from keys file (optional)")
	keysFile := fs.String("keys-file", "", "Path to keys.yaml with tenant configs (optional)")
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

	var emb embed.Embedder
	switch cfg.RAGEmbedder {
	case "openai":
		emb = embed.NewOpenAIEmbedder(cfg.OpenAIAPIKey, nil)
	case "onnx":
		onnxEmb, err := embed.NewONNXEmbedder(cfg.ONNXModelPath, cfg.ONNXLibDir)
		if err != nil {
			log.Fatalf("onnx embedder: %v", err)
		}
		emb = onnxEmb
	default: // "voyage"
		emb = embed.NewVoyageEmbedder(cfg.VoyageAPIKey, nil)
	}

	st, err := kpg.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	var (
		tenantID string
		source   = "jira"
	)

	if *tenantName != "" {
		// Multi-tenant mode: load config from keys file.
		if *keysFile == "" {
			log.Fatalf("index: --keys-file is required when --tenant is specified")
		}
		tenantConfigs, err := tenant.LoadTenantsFromFile(*keysFile)
		if err != nil {
			log.Fatalf("index: load keys file: %v", err)
		}

		var tc *tenant.Config
		for i := range tenantConfigs {
			if tenantConfigs[i].Name == *tenantName {
				tc = &tenantConfigs[i]
				break
			}
		}
		if tc == nil {
			log.Fatalf("index: tenant %q not found in %s", *tenantName, *keysFile)
		}

		baseURL := tc.TrackerConfig["base_url"]
		if baseURL == "" {
			baseURL = cfg.JiraBaseURL
		}
		email := tc.TrackerConfig["email"]
		if email == "" {
			email = cfg.JiraEmail
		}
		token := tc.TrackerConfig["api_token"]
		if token == "" {
			token = cfg.JiraAPIToken
		}
		authType := tc.TrackerConfig["auth_type"]
		if authType == "" {
			authType = cfg.JiraAuthType
		}

		jc := jiratracker.NewHTTPClient(baseURL, email, token, authType, nil)
		indexer := kindex.New(jc, emb, st)

		tenantID = tc.Name
		start := time.Now()
		n, err := indexer.Reindex(ctx, tenantID, source, *projectKey)
		if err != nil {
			log.Fatalf("reindex: %v", err)
		}
		log.Printf("indexed %d docs for tenant %q in %s", n, tenantID, time.Since(start))
	} else {
		// Legacy single-tenant mode from env vars.
		jc := jiratracker.NewHTTPClient(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, cfg.JiraAuthType, nil)
		indexer := kindex.New(jc, emb, st)

		start := time.Now()
		n, err := indexer.Reindex(ctx, tenantID, source, *projectKey)
		if err != nil {
			log.Fatalf("reindex: %v", err)
		}
		log.Printf("indexed %d docs in %s", n, time.Since(start))
	}
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

	if err := kpg.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Printf("migrations applied")
}
