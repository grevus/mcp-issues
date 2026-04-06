package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/grevus/mcp-jira/internal/config"
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
