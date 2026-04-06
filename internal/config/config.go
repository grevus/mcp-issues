package config

import (
	"fmt"
	"os"
)

// Mode определяет режим запуска сервера.
type Mode string

const (
	ModeStdio Mode = "stdio"
	ModeHTTP  Mode = "http"
	ModeIndex Mode = "index"
)

// Config содержит всю конфигурацию приложения.
type Config struct {
	Mode         Mode
	JiraBaseURL  string
	JiraEmail    string
	JiraAPIToken string
	DatabaseURL  string
	RAGEmbedder  string // "voyage" или "openai"
	VoyageAPIKey string
	OpenAIAPIKey string
	MCPAPIKey    string // только для http
	MCPAddr      string // только для http, default ":8080"
}

// Load читает переменные окружения и возвращает Config для указанного mode.
// Общие обязательные переменные: JIRA_BASE_URL, JIRA_EMAIL, JIRA_API_TOKEN, DATABASE_URL.
func Load(mode Mode) (*Config, error) {
	required := []struct {
		env   string
		value *string
	}{
		{"JIRA_BASE_URL", nil},
		{"JIRA_EMAIL", nil},
		{"JIRA_API_TOKEN", nil},
		{"DATABASE_URL", nil},
	}

	values := make([]string, len(required))
	for i, r := range required {
		v := os.Getenv(r.env)
		if v == "" {
			return nil, fmt.Errorf("config: %s is required", r.env)
		}
		values[i] = v
	}

	embedder := os.Getenv("RAG_EMBEDDER")
	if embedder == "" {
		embedder = "voyage"
	}
	if embedder != "voyage" && embedder != "openai" {
		return nil, fmt.Errorf("config: RAG_EMBEDDER must be \"voyage\" or \"openai\", got %q", embedder)
	}

	var voyageKey, openaiKey string
	switch embedder {
	case "voyage":
		voyageKey = os.Getenv("VOYAGE_API_KEY")
		if voyageKey == "" {
			return nil, fmt.Errorf("config: VOYAGE_API_KEY is required when RAG_EMBEDDER=voyage")
		}
	case "openai":
		openaiKey = os.Getenv("OPENAI_API_KEY")
		if openaiKey == "" {
			return nil, fmt.Errorf("config: OPENAI_API_KEY is required when RAG_EMBEDDER=openai")
		}
	}

	var mcpAPIKey, mcpAddr string
	if mode == ModeHTTP {
		mcpAPIKey = os.Getenv("MCP_API_KEY")
		if mcpAPIKey == "" {
			return nil, fmt.Errorf("config: MCP_API_KEY is required for http mode")
		}
		mcpAddr = os.Getenv("MCP_ADDR")
		if mcpAddr == "" {
			mcpAddr = ":8080"
		}
	}

	return &Config{
		Mode:         mode,
		JiraBaseURL:  values[0],
		JiraEmail:    values[1],
		JiraAPIToken: values[2],
		DatabaseURL:  values[3],
		RAGEmbedder:  embedder,
		VoyageAPIKey: voyageKey,
		OpenAIAPIKey: openaiKey,
		MCPAPIKey:    mcpAPIKey,
		MCPAddr:      mcpAddr,
	}, nil
}
