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
	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/knowledge/embed"
	kpg "github.com/grevus/mcp-jira/internal/knowledge/pgvector"
	ksqlite "github.com/grevus/mcp-jira/internal/knowledge/sqlite"
	"github.com/grevus/mcp-jira/internal/knowledge/retriever"
	"github.com/grevus/mcp-jira/internal/register"
	"github.com/grevus/mcp-jira/internal/tenant"
	"github.com/grevus/mcp-jira/internal/tracker"
	jiratracker "github.com/grevus/mcp-jira/internal/tracker/jira"
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

	// Embedder: switch by cfg.RAGEmbedder.
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

	// Knowledge store.
	var st knowledge.Store
	switch cfg.KnowledgeStore {
	case "pgvector":
		pgst, err := kpg.New(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("pgvector store: %v", err)
		}
		st = pgst
	default: // "sqlite"
		sqst, err := ksqlite.New(ctx, cfg.SQLitePath)
		if err != nil {
			log.Fatalf("sqlite store: %v", err)
		}
		st = sqst
	}
	defer st.Close()

	// Build tenant Registry.
	reg := tenant.NewRegistry()

	if cfg.MCPKeysFile != "" {
		// Multi-tenant mode: load tenant configs from keys file.
		tenantConfigs, err := tenant.LoadTenantsFromFile(cfg.MCPKeysFile)
		if err != nil {
			log.Fatalf("tenant: load keys file: %v", err)
		}
		for _, tc := range tenantConfigs {
			var prov tracker.Provider
			switch tc.TrackerType {
			case "jira", "":
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
				prov = jiratracker.NewHTTPClient(baseURL, email, token, authType, nil)
			default:
				log.Fatalf("unknown tracker type %q for tenant %q", tc.TrackerType, tc.Name)
			}
			ret := retriever.New(emb, st, tc.Name)
			reg.Register(tc.Name, &tenant.Tenant{
				Config:    tc,
				Provider:  prov,
				Knowledge: st,
				Retriever: ret,
			})
		}
	} else {
		// Single-tenant mode: use env vars, register under "default".
		jc := jiratracker.NewHTTPClient(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, cfg.JiraAuthType, nil)
		ret := retriever.New(emb, st, "")
		reg.Register("default", &tenant.Tenant{
			Config:    tenant.Config{Name: "default"},
			Provider:  jc,
			Knowledge: st,
			Retriever: ret,
		})
	}

	// MCP server.
	srv := mcp.NewServer(&mcp.Implementation{Name: "mcp-jira", Version: "0.1.0"}, nil)
	register.Register(srv, reg)

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

		if cfg.MCPKeysFile != "" {
			// Multi-tenant: load keys for auth middleware from the same file.
			keys, err := auth.LoadKeys(cfg.MCPKeysFile)
			if err != nil {
				log.Fatalf("auth: load keys: %v", err)
			}
			e.Use(echo.WrapMiddleware(auth.MultiKeyMiddleware(keys)))
		} else {
			e.Use(echo.WrapMiddleware(auth.Middleware(cfg.MCPAPIKey)))
		}

		mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		e.Any("/mcp", echo.WrapHandler(mcpHandler))

		log.Printf("mcp-jira: starting HTTP transport on %s", cfg.MCPAddr)
		if err := e.Start(cfg.MCPAddr); err != nil {
			log.Fatalf("http transport: %v", err)
		}
	}
}
