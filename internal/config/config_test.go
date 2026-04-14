package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/grevus/mcp-jira/internal/config"
	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "user@example.com")
	t.Setenv("JIRA_API_TOKEN", "secret-token")
	// Most tests exercise pgvector mode historically; keep that by default.
	// Tests that care about the default sqlite path set KNOWLEDGE_STORE explicitly.
	t.Setenv("KNOWLEDGE_STORE", "pgvector")
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

func TestLoad_DotEnvFile(t *testing.T) {
	t.Chdir(t.TempDir())

	envContent := strings.Join([]string{
		"# comment",
		"JIRA_BASE_URL=https://dot.atlassian.net",
		"JIRA_EMAIL=dot@example.com",
		"JIRA_API_TOKEN=dot-token",
		"KNOWLEDGE_STORE=pgvector",
		"DATABASE_URL=postgres://localhost/dotenv",
		"VOYAGE_API_KEY=dot-voyage-key",
	}, "\n")
	require.NoError(t, os.WriteFile(".env", []byte(envContent), 0o600))

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "https://dot.atlassian.net", cfg.JiraBaseURL)
	require.Equal(t, "dot@example.com", cfg.JiraEmail)
	require.Equal(t, "dot-token", cfg.JiraAPIToken)
	require.Equal(t, "postgres://localhost/dotenv", cfg.DatabaseURL)
	require.Equal(t, "dot-voyage-key", cfg.VoyageAPIKey)
}

func TestLoad_EnvOverridesDotEnv(t *testing.T) {
	t.Chdir(t.TempDir())

	require.NoError(t, os.WriteFile(".env", []byte("JIRA_BASE_URL=https://from-file.atlassian.net\n"), 0o600))
	setRequiredEnv(t) // выставляет JIRA_BASE_URL=https://example.atlassian.net

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "https://example.atlassian.net", cfg.JiraBaseURL)
}

func TestLoad_BearerAuthNoEmailRequired(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JIRA_AUTH_TYPE", "bearer")
	t.Setenv("JIRA_EMAIL", "") // email не нужен при bearer

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "bearer", cfg.JiraAuthType)
	require.Empty(t, cfg.JiraEmail)
}

func TestLoad_BearerAuthInvalidType(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JIRA_AUTH_TYPE", "oauth")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "JIRA_AUTH_TYPE")
}

func TestLoad_BasicAuthDefaultsWhenUnset(t *testing.T) {
	setRequiredEnv(t)
	// JIRA_AUTH_TYPE не задан — должен дефолтиться в "basic"

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "basic", cfg.JiraAuthType)
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

func TestLoad_KnowledgeStore_DefaultsToSqlite(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("KNOWLEDGE_STORE", "")
	t.Setenv("DATABASE_URL", "")

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "sqlite", cfg.KnowledgeStore)
	require.NotEmpty(t, cfg.SQLitePath)
	require.Empty(t, cfg.DatabaseURL)
}

func TestLoad_KnowledgeStore_SqliteCustomPath(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("KNOWLEDGE_STORE", "sqlite")
	t.Setenv("SQLITE_PATH", "/custom/path/knowledge.db")
	t.Setenv("DATABASE_URL", "")

	cfg, err := config.Load(config.ModeStdio)
	require.NoError(t, err)
	require.Equal(t, "sqlite", cfg.KnowledgeStore)
	require.Equal(t, "/custom/path/knowledge.db", cfg.SQLitePath)
}

func TestLoad_KnowledgeStore_PgvectorRequiresDatabaseURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("KNOWLEDGE_STORE", "pgvector")
	t.Setenv("DATABASE_URL", "")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_KnowledgeStore_Invalid(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("KNOWLEDGE_STORE", "redis")

	cfg, err := config.Load(config.ModeStdio)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "KNOWLEDGE_STORE")
}
