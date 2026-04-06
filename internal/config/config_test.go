package config_test

import (
	"testing"

	"github.com/grevus/mcp-jira/internal/config"
	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "user@example.com")
	t.Setenv("JIRA_API_TOKEN", "secret-token")
	t.Setenv("DATABASE_URL", "postgres://localhost/testdb")
	t.Setenv("VOYAGE_API_KEY", "voyage-test-key")
}

func TestLoad_HappyPath(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, config.ModeStdio, cfg.Mode)
	require.Equal(t, "https://example.atlassian.net", cfg.JiraBaseURL)
	require.Equal(t, "user@example.com", cfg.JiraEmail)
	require.Equal(t, "secret-token", cfg.JiraAPIToken)
	require.Equal(t, "postgres://localhost/testdb", cfg.DatabaseURL)
}

func TestLoad_MissingJiraBaseURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JIRA_BASE_URL", "")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "JIRA_BASE_URL")
}

func TestLoad_MissingJiraEmail(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JIRA_EMAIL", "")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "JIRA_EMAIL")
}

func TestLoad_MissingJiraAPIToken(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JIRA_API_TOKEN", "")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "JIRA_API_TOKEN")
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DATABASE_URL", "")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_EmbedderDefault(t *testing.T) {
	setRequiredEnv(t)
	// RAG_EMBEDDER не задан — должен использоваться "voyage" по умолчанию.

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "voyage", cfg.RAGEmbedder)
	require.Equal(t, "voyage-test-key", cfg.VoyageAPIKey)
	require.Empty(t, cfg.OpenAIAPIKey)
}

func TestLoad_EmbedderVoyageExplicit(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("RAG_EMBEDDER", "voyage")

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "voyage", cfg.RAGEmbedder)
	require.Equal(t, "voyage-test-key", cfg.VoyageAPIKey)
	require.Empty(t, cfg.OpenAIAPIKey)
}

func TestLoad_EmbedderOpenAI(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("RAG_EMBEDDER", "openai")
	t.Setenv("VOYAGE_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openai-test-key")

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "openai", cfg.RAGEmbedder)
	require.Equal(t, "openai-test-key", cfg.OpenAIAPIKey)
	require.Empty(t, cfg.VoyageAPIKey)
}

func TestLoad_EmbedderUnknown(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("RAG_EMBEDDER", "cohere")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "RAG_EMBEDDER")
	require.Contains(t, err.Error(), "cohere")
}

func TestLoad_EmbedderVoyageMissingKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("VOYAGE_API_KEY", "")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "VOYAGE_API_KEY")
}

func TestLoad_EmbedderOpenAIMissingKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("RAG_EMBEDDER", "openai")
	t.Setenv("VOYAGE_API_KEY", "")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestLoad_HTTP_RequiresMCPAPIKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MCP_API_KEY", "")

	cfg, err := config.Load(config.ModeHTTP)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "MCP_API_KEY")
}

func TestLoad_HTTP_HappyPath(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MCP_API_KEY", "secret")
	t.Setenv("MCP_ADDR", "")

	cfg, err := config.Load(config.ModeHTTP)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "secret", cfg.MCPAPIKey)
	require.Equal(t, ":8080", cfg.MCPAddr)
}

func TestLoad_HTTP_CustomMCPAddr(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MCP_API_KEY", "secret")
	t.Setenv("MCP_ADDR", ":9090")

	cfg, err := config.Load(config.ModeHTTP)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, ":9090", cfg.MCPAddr)
}

func TestLoad_Stdio_IgnoresMCPAPIKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MCP_API_KEY", "")
	t.Setenv("MCP_ADDR", ":9090")

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "", cfg.MCPAPIKey)
	require.Equal(t, "", cfg.MCPAddr)
}

func TestLoad_Index_IgnoresMCPAPIKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MCP_API_KEY", "")
	t.Setenv("MCP_ADDR", ":9090")

	cfg, err := config.Load(config.ModeIndex)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "", cfg.MCPAPIKey)
	require.Equal(t, "", cfg.MCPAddr)
}
