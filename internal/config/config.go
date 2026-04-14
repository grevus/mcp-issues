package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv читает файл path в формате KEY=VALUE и устанавливает переменные окружения,
// которые ещё не заданы. Если файл не найден — молча игнорирует.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// Mode определяет режим запуска сервера.
type Mode string

const (
	ModeStdio Mode = "stdio"
	ModeHTTP  Mode = "http"
	ModeIndex Mode = "index"
)

// Config содержит всю конфигурацию приложения.
type Config struct {
	Mode           Mode
	JiraBaseURL    string
	JiraEmail      string
	JiraAPIToken   string
	JiraAuthType   string // "basic" (default) | "bearer" (Jira DC PAT)
	KnowledgeStore string // "sqlite" (default) | "pgvector"
	DatabaseURL    string // only for pgvector
	SQLitePath     string // only for sqlite, default "~/.mcp-jira/knowledge.db"
	RAGEmbedder    string // "voyage" | "openai" | "onnx"
	VoyageAPIKey   string
	OpenAIAPIKey   string
	ONNXModelPath  string // path to directory with model.onnx (onnx only)
	ONNXLibDir     string // path to onnxruntime lib directory (onnx, optional)
	MCPAPIKey      string // http mode only (single-key)
	MCPAddr        string // http mode only, default ":8080"
	MCPKeysFile    string // path to YAML multi-tenant keys file (optional)
}

// Load читает переменные окружения и возвращает Config для указанного mode.
// В multi-tenant режиме (MCP_KEYS_FILE задан) Jira env опциональны —
// credentials берутся из YAML per-tenant.
// KNOWLEDGE_STORE=sqlite (default) не требует DATABASE_URL; pgvector требует.
func Load(mode Mode) (*Config, error) {
	LoadDotEnv(".env")

	// Knowledge store selection.
	knowledgeStore := os.Getenv("KNOWLEDGE_STORE")
	if knowledgeStore == "" {
		knowledgeStore = "sqlite"
	}
	if knowledgeStore != "sqlite" && knowledgeStore != "pgvector" {
		return nil, fmt.Errorf("config: KNOWLEDGE_STORE must be \"sqlite\" or \"pgvector\", got %q", knowledgeStore)
	}

	var dbURL, sqlitePath string
	if knowledgeStore == "pgvector" {
		dbURL = os.Getenv("DATABASE_URL")
		if dbURL == "" {
			return nil, fmt.Errorf("config: DATABASE_URL is required when KNOWLEDGE_STORE=pgvector")
		}
	} else {
		sqlitePath = os.Getenv("SQLITE_PATH")
		if sqlitePath == "" {
			home, _ := os.UserHomeDir()
			sqlitePath = home + "/.mcp-jira/knowledge.db"
		}
	}

	// Определяем, есть ли multi-tenant keys file (нужно раньше, чтобы
	// понять, обязательны ли Jira env).
	mcpKeysFile := os.Getenv("MCP_KEYS_FILE")

	authType := os.Getenv("JIRA_AUTH_TYPE")
	if authType == "" {
		authType = "basic"
	}
	if authType != "basic" && authType != "bearer" {
		return nil, fmt.Errorf("config: JIRA_AUTH_TYPE must be \"basic\" or \"bearer\", got %q", authType)
	}

	// Jira env обязательны только в single-tenant режиме (без MCP_KEYS_FILE).
	values := make(map[string]string)
	if mcpKeysFile == "" {
		required := []string{"JIRA_BASE_URL", "JIRA_API_TOKEN"}
		if authType == "basic" {
			required = append(required, "JIRA_EMAIL")
		}
		for _, env := range required {
			v := os.Getenv(env)
			if v == "" {
				return nil, fmt.Errorf("config: %s is required", env)
			}
			values[env] = v
		}
	}
	// Читаем Jira env как fallback (могут быть заданы даже в multi-tenant).
	values["JIRA_BASE_URL"] = os.Getenv("JIRA_BASE_URL")
	values["JIRA_EMAIL"] = os.Getenv("JIRA_EMAIL")
	values["JIRA_API_TOKEN"] = os.Getenv("JIRA_API_TOKEN")

	embedder := os.Getenv("RAG_EMBEDDER")
	if embedder == "" {
		embedder = "voyage"
	}
	if embedder != "voyage" && embedder != "openai" && embedder != "onnx" {
		return nil, fmt.Errorf("config: RAG_EMBEDDER must be \"voyage\", \"openai\" or \"onnx\", got %q", embedder)
	}

	var voyageKey, openaiKey, onnxModelPath, onnxLibDir string
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
	case "onnx":
		onnxModelPath = os.Getenv("ONNX_MODEL_PATH")
		if onnxModelPath == "" {
			return nil, fmt.Errorf("config: ONNX_MODEL_PATH is required when RAG_EMBEDDER=onnx")
		}
		onnxLibDir = os.Getenv("ONNX_LIB_DIR") // опционально
	}

	var mcpAPIKey, mcpAddr string
	if mode == ModeHTTP {
		if mcpKeysFile == "" {
			mcpAPIKey = os.Getenv("MCP_API_KEY")
			if mcpAPIKey == "" {
				return nil, fmt.Errorf("config: MCP_API_KEY or MCP_KEYS_FILE is required for http mode")
			}
		}
		mcpAddr = os.Getenv("MCP_ADDR")
		if mcpAddr == "" {
			mcpAddr = ":8080"
		}
	}

	return &Config{
		Mode:           mode,
		JiraBaseURL:    values["JIRA_BASE_URL"],
		JiraEmail:      values["JIRA_EMAIL"],
		JiraAPIToken:   values["JIRA_API_TOKEN"],
		JiraAuthType:   authType,
		KnowledgeStore: knowledgeStore,
		DatabaseURL:    dbURL,
		SQLitePath:     sqlitePath,
		RAGEmbedder:    embedder,
		VoyageAPIKey:   voyageKey,
		OpenAIAPIKey:   openaiKey,
		ONNXModelPath:  onnxModelPath,
		ONNXLibDir:     onnxLibDir,
		MCPAPIKey:      mcpAPIKey,
		MCPAddr:        mcpAddr,
		MCPKeysFile:    mcpKeysFile,
	}, nil
}
